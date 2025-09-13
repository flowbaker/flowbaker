package initialization

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/flowbaker/flowbaker/pkg/clients/flowbaker"
	"github.com/flowbaker/flowbaker/pkg/domain"
)

// RegisterExecutor registers the executor with the API and returns verification code
func RegisterExecutor(executorName, address string, keys domain.CryptoKeys, apiBaseURL string) (string, error) {
	client := flowbaker.NewClient(
		flowbaker.WithBaseURL(apiBaseURL),
	)

	req := &flowbaker.CreateExecutorRegistrationRequest{
		ExecutorName:     executorName,
		Address:          address,
		X25519PublicKey:  keys.X25519Public,
		Ed25519PublicKey: keys.Ed25519Public,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.CreateExecutorRegistration(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to register executor: %w", err)
	}

	return resp.VerificationCode, nil
}

type verificationSuccessMsg struct {
	executorID          string
	workspaceAssignment domain.WorkspaceAssignment
}

type verificationErrorMsg struct {
	err error
}

type verificationModel struct {
	spinner          spinner.Model
	verificationCode string
	done             bool
	err              error
	result           *verificationSuccessMsg
}

func (m verificationModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m verificationModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.err = fmt.Errorf("verification cancelled")
			m.done = true
			return m, tea.Quit
		}
	case verificationSuccessMsg:
		m.result = &msg
		m.done = true
		return m, tea.Quit
	case verificationErrorMsg:
		m.err = msg.err
		m.done = true
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m verificationModel) View() string {
	if m.done {
		return ""
	}

	style := lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return fmt.Sprintf("\n%s %s\n\n", m.spinner.View(), style.Render("Waiting for workspace registration..."))
}

type WaitForVerificationParams struct {
	ExecutorName                 string
	VerificationCode             string
	Keys                         domain.CryptoKeys
	APIBaseURL                   string
	WorkspaceRegistrationManager domain.WorkspaceRegistrationManager
	EnableStaticPasscode         bool
	StaticPasscode               string
}

type WaitForVerificationResult struct {
	ExecutorID          string
	WorkspaceAssignment domain.WorkspaceAssignment
}

func WaitForVerification(params WaitForVerificationParams) (WaitForVerificationResult, error) {
	passcode, err := GeneratePasscode()
	if err != nil {
		return WaitForVerificationResult{}, fmt.Errorf("failed to generate passcode: %w", err)
	}

	if params.EnableStaticPasscode {
		passcode = params.StaticPasscode
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	model := verificationModel{
		spinner:          s,
		verificationCode: params.VerificationCode,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	program := tea.NewProgram(model, tea.WithContext(ctx))

	err = params.WorkspaceRegistrationManager.AddPasscode(context.Background(), domain.AddPasscodeParams{
		Passcode: passcode,
		OnSuccess: func(ctx context.Context, params domain.RegisterWorkspaceParams) error {
			program.Send(verificationSuccessMsg{
				executorID: params.ExecutorID,
				workspaceAssignment: domain.WorkspaceAssignment{
					WorkspaceID:   params.Assignment.WorkspaceID,
					WorkspaceName: params.Assignment.WorkspaceName,
					WorkspaceSlug: params.Assignment.WorkspaceSlug,
					APIPublicKey:  params.Assignment.APIPublicKey,
				},
			})
			return nil
		},
	})
	if err != nil {
		return WaitForVerificationResult{}, fmt.Errorf("failed to register callback: %w", err)
	}

	fmt.Printf("🔑 Registration passcode: %s\n", passcode)
	fmt.Println("💡 Use this passcode when registering workspaces via the API")
	fmt.Println()

	finalModel, err := program.Run()
	if err != nil {
		return WaitForVerificationResult{}, fmt.Errorf("failed to run verification interface: %w", err)
	}

	final := finalModel.(verificationModel)
	if final.err != nil {
		return WaitForVerificationResult{}, final.err
	}

	if final.result == nil {
		return WaitForVerificationResult{}, fmt.Errorf("verification timeout after 10 minutes")
	}

	fmt.Println("✅ Executor registration verified!")
	return WaitForVerificationResult{
		ExecutorID:          final.result.executorID,
		WorkspaceAssignment: final.result.workspaceAssignment,
	}, nil
}
