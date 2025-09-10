package initialization

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/flowbaker/flowbaker/pkg/clients/flowbaker"
)

// RegisterExecutor registers the executor with the API and returns verification code
func RegisterExecutor(executorName, address string, keys CryptoKeys, apiBaseURL string) (string, error) {
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

type verificationModel struct {
	spinner          spinner.Model
	client           *flowbaker.Client
	verificationCode string
	done             bool
	result           *verificationResult
	err              error
}

type verificationResult struct {
	executorID           string
	workspaceIDs         []string
	workspaceNames       []string
	workspaceAssignments []WorkspaceAPIKey
}

type statusChecked struct {
	status *flowbaker.RegistrationStatusResponse
	err    error
}

func (m verificationModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.checkStatus())
}

func (m verificationModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.err = fmt.Errorf("verification cancelled")
			m.done = true
			return m, tea.Quit
		}
	case statusChecked:
		if msg.err != nil {
			// Continue polling on error
			return m, tea.Tick(5*time.Second, func(time.Time) tea.Msg {
				return m.checkStatus()()
			})
		}

		switch msg.status.Status {
		case "verified":
			if msg.status.Executor != nil {
				workspaceAssignments := make([]WorkspaceAPIKey, len(msg.status.WorkspaceAssignments))
				for i, assignment := range msg.status.WorkspaceAssignments {
					workspaceAssignments[i] = WorkspaceAPIKey{
						WorkspaceID:  assignment.WorkspaceID,
						APIPublicKey: assignment.APIPublicKey,
					}
				}

				m.result = &verificationResult{
					executorID:           msg.status.Executor.ID,
					workspaceIDs:         msg.status.Executor.WorkspaceIDs,
					workspaceNames:       msg.status.WorkspaceNames,
					workspaceAssignments: workspaceAssignments,
				}
				m.done = true
				return m, tea.Quit
			}
			m.err = fmt.Errorf("executor data not available in verification response")
			m.done = true
			return m, tea.Quit
		case "not_found":
			m.err = fmt.Errorf("registration not found or expired: %s", msg.status.Message)
			m.done = true
			return m, tea.Quit
		case "pending":
			// Continue polling
			return m, tea.Tick(5*time.Second, func(time.Time) tea.Msg {
				return m.checkStatus()()
			})
		default:
			// Continue polling for unknown status
			return m, tea.Tick(5*time.Second, func(time.Time) tea.Msg {
				return m.checkStatus()()
			})
		}
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
	return fmt.Sprintf("\n%s %s\n\n", m.spinner.View(), style.Render("Waiting for executor verification..."))
}

func (m verificationModel) checkStatus() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		status, err := m.client.GetExecutorRegistrationStatus(ctx, m.verificationCode)
		return statusChecked{status: status, err: err}
	}
}

// WaitForVerification waits for the executor to be verified via the frontend
func WaitForVerification(executorName, verificationCode string, keys CryptoKeys, apiBaseURL string) (string, []string, []string, []WorkspaceAPIKey, error) {
	client := flowbaker.NewClient(
		flowbaker.WithBaseURL(apiBaseURL),
	)

	// Initialize spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	// Create verification model
	model := verificationModel{
		spinner:          s,
		client:           client,
		verificationCode: verificationCode,
	}

	// Run the spinner with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Create a program with context
	p := tea.NewProgram(model, tea.WithContext(ctx))

	// Run the program
	finalModel, err := p.Run()
	if err != nil {
		return "", nil, nil, nil, fmt.Errorf("failed to run verification interface: %w", err)
	}

	// Check the final state
	final := finalModel.(verificationModel)
	if final.err != nil {
		return "", nil, nil, nil, final.err
	}

	if final.result != nil {
		fmt.Println("✅ Executor registration verified!")
		return final.result.executorID, final.result.workspaceIDs, final.result.workspaceNames, final.result.workspaceAssignments, nil
	}

	return "", nil, nil, nil, fmt.Errorf("verification timeout after 10 minutes")
}
