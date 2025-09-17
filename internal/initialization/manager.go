package initialization

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/flowbaker/flowbaker/pkg/domain"
)

const flowbakerLogo = `       ...........................*####=--:.       
    -...--------------.....:----..@%%%%%@@@@#.-    
  ==@@#...-------........-..----..@%%%%%%%%%%@@%*  
 :#%%%@@*...----..:%@@@@@@#.----.*@%%%%%%%%%@@*... 
-#%%%%%@@@-...--.@@@@%%%%@*.---..@@%%%%%%%@@#...-. 
-%%%%%%%%@@%......%@%%%%%@*.--:.-@%%%%%%@@%...:---.
#%%%%%%%%%%@@%.....#@@%%%@*.--..@@%%%%@@%...:-----.
#%%%%%%%%%%%%@@#.....@@%%@*.-..%@%%%@@@-...-------.
#@@@@@@@@%%%%%%@@*....@@%@*...#@%%%@@*.......-----.
.....*#%@@@@@@@%@@@=...@%@*..*@%%@@#......*=..----.
.::.........*%@@@@@@@..*@@=.#@@@@%.....+%@@@*.:---.
.---------......*%@@@@#.@@.%@@@%.....#@@@%%@@..---.
.---................%@@@*@%@@*...-#@@@%%%%%%@#.---.
.---.%###########*+:..*@%%%%+%@@@@@@@@@@@@@@@@.---.
.---.@@@@@@@@@@@@@@@@%=%%%%@+..-+*###########@.---.
.---.#@%%%%%%@@@%*...+@@%@+@@@%................---.
.---..@%%%@@@#.....%@@@#.@@.#@@@@%*......---------.
.---..%@@@@*.....%@@@@*.:@@-..@@@@@@@#+............
.----..%%+.....#@@%@@...+@@@...+@@@%@@@@@@%**-.....
.-----.......*@@%%%@+...+@%@@....*@@%%%%%@@@@@@@@@#
.-------...=@@@%%%@*..-.+@%%@@.....#@@%%%%%%%%%%%%#
.-----....%@@%%%%@%..--.+@%%%@@*.....%@@%%%%%%%%%%+
.---:...%@@%%%%%%@..---.+@%%%%@@#......@@@%%%%%%%%-
 .-...#@@%%%%%%%@%..---.+@%%%%@@@@.--...=@@@%%%%%#-
 ...*@@%%%%%%%%%@=.----.#@@@@@@#...----...*@@%%%#: 
  *%@@%%%%%%%%%%@..----..:........-------...#@@==  
    -.#@@@%%%%%@%.:-----.....-------------:...-    
       .:--=####=...........................       `

func showWelcome() {
	// Create styles
	logoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true).
		Align(lipgloss.Center)

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true).
		Align(lipgloss.Center).
		MarginTop(1).
		MarginBottom(1)

	subtitleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Italic(true).
		Align(lipgloss.Center).
		MarginBottom(2)

	// Display welcome screen
	fmt.Println(logoStyle.Render(flowbakerLogo))
	fmt.Println(titleStyle.Render("✨ Welcome to Flowbaker!"))
	fmt.Println(subtitleStyle.Render("Let's get your automation magic ready to roll..."))
}

func getPublicIP() (string, error) {
	// Try multiple services in case one is down
	services := []string{
		"https://api.ipify.org",
		"https://icanhazip.com",
		"https://ipinfo.io/ip",
	}

	for _, service := range services {
		resp, err := http.Get(service)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				continue
			}
			ip := strings.TrimSpace(string(body))
			if ip != "" {
				return ip, nil
			}
		}
	}

	return "", fmt.Errorf("failed to detect public IP address from any service")
}

func detectVPN() bool {
	// Check for VPN network interfaces
	interfaces, err := net.Interfaces()
	if err != nil {
		return false
	}

	vpnInterfaces := []string{
		"wg",    // WireGuard (wg0, wg1, etc.)
		"tun",   // OpenVPN, other tunnel interfaces (tun0, tun1, etc.)
		"tap",   // TAP interfaces (tap0, tap1, etc.)
		"ppp",   // Point-to-Point Protocol (ppp0, ppp1, etc.)
		"ipsec", // IPSec interfaces
		"vpn",   // Generic VPN interface names
	}

	for _, iface := range interfaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		name := strings.ToLower(iface.Name)
		for _, vpnPrefix := range vpnInterfaces {
			if strings.HasPrefix(name, vpnPrefix) {
				return true
			}
		}
	}

	return false
}

func collectExecutorConfig() (string, string, error) {
	var executorName, address string

	// Get default values from environment or fallbacks
	defaultName := os.Getenv("FLOWBAKER_EXECUTOR_NAME")
	if defaultName == "" {
		defaultName = GenerateExecutorName()
	}

	defaultAddress := os.Getenv("FLOWBAKER_EXECUTOR_ADDRESS")
	if defaultAddress == "" {
		publicIP, err := getPublicIP()
		if err != nil {
			return "", "", fmt.Errorf("failed to detect public IP address: %w", err)
		}
		defaultAddress = fmt.Sprintf("http://%s:8081", publicIP)

		// Check for VPN and warn user
		if detectVPN() {
			var continueSetup bool

			warningForm := huh.NewForm(
				huh.NewGroup(
					huh.NewConfirm().
						Title("⚠️  VPN Detected").
						Description("VPN network interface detected. This may prevent external services from reaching your executor. Continue anyway?").
						Value(&continueSetup),
				),
			)

			if err := warningForm.Run(); err != nil {
				return "", "", err
			}

			if !continueSetup {
				return "", "", fmt.Errorf("setup cancelled by user")
			}
		}
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Executor Name").
				Description("A friendly name for your executor").
				Value(&executorName).
				Placeholder(defaultName),

			huh.NewInput().
				Title("Executor Address").
				Description("Full URL where your executor will listen (e.g., http://1.2.3.4:8081 or https://mydomain.com:8081)").
				Value(&address).
				Placeholder(defaultAddress),
		),
	)

	err := form.Run()
	if err != nil {
		return "", "", err
	}

	// Use defaults if user didn't enter anything
	if executorName == "" {
		executorName = defaultName
	}
	if address == "" {
		address = defaultAddress
	}

	return executorName, address, nil
}

type RunFirstTimeSetupParams struct {
	ConfigManager       domain.ConfigManager
	RegistrationManager domain.WorkspaceRegistrationManager
}

func RunFirstTimeSetup(ctx context.Context, params RunFirstTimeSetupParams) error {
	showWelcome()

	executorName, address, err := collectExecutorConfig()
	if err != nil {
		return fmt.Errorf("failed to collect executor configuration: %w", err)
	}

	keys, err := GenerateAllKeys()
	if err != nil {
		return fmt.Errorf("failed to generate keys: %w", err)
	}

	apiURL := getAPIURL()

	fmt.Println("📡 Registering with Flowbaker...")
	verificationCode, err := RegisterExecutor(executorName, address, keys, apiURL)
	if err != nil {
		return fmt.Errorf("failed to register executor: %w", err)
	}

	frontendURL := GetVerificationURL(apiURL)
	connectionURL := fmt.Sprintf("%s/executors/verify?code=%s", frontendURL, verificationCode)
	fmt.Println()
	fmt.Println("🔗 Connect your executor:")
	fmt.Println()
	fmt.Printf("   %s\n", connectionURL)
	fmt.Println()
	fmt.Println("⏳ Waiting for connection...")

	enableStaticPasscode := os.Getenv("FLOWBAKER_ENABLE_STATIC_PASSCODE") == "true"
	staticPasscode := os.Getenv("FLOWBAKER_STATIC_PASSCODE")

	p := WaitForVerificationParams{
		ExecutorName:                 executorName,
		VerificationCode:             verificationCode,
		Keys:                         keys,
		APIBaseURL:                   apiURL,
		WorkspaceRegistrationManager: params.RegistrationManager,
		EnableStaticPasscode:         enableStaticPasscode,
		StaticPasscode:               staticPasscode,
	}

	result, err := WaitForVerification(p)
	if err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	config := &domain.ExecutorConfig{
		ExecutorID:                  result.ExecutorID,
		ExecutorName:                executorName,
		Address:                     address,
		APIBaseURL:                  apiURL,
		X25519PrivateKey:            keys.X25519Private,
		X25519PublicKey:             keys.X25519Public,
		Ed25519PrivateKey:           keys.Ed25519Private,
		Ed25519PublicKey:            keys.Ed25519Public,
		SetupComplete:               true,
		WorkspaceAssignments:        []domain.WorkspaceAssignment{result.WorkspaceAssignment},
		EnableWorkspaceRegistration: true,
		EnableStaticPasscode:        enableStaticPasscode,
		StaticPasscode:              staticPasscode,
		LastConnected:               time.Now().Format(time.RFC3339),
	}

	if err := params.ConfigManager.SaveConfig(ctx, *config); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	workspaceNames := make([]string, len(config.WorkspaceAssignments))

	for i, assignment := range config.WorkspaceAssignments {
		workspaceNames[i] = assignment.WorkspaceName
	}

	fmt.Println()
	fmt.Printf("✅ Connected to %d workspace(s): %s\n", len(config.WorkspaceAssignments), strings.Join(workspaceNames, ", "))
	fmt.Println("💾 Configuration saved")

	return nil
}

func getAPIURL() string {
	if url := os.Getenv("FLOWBAKER_API_URL"); url != "" {
		return url
	}
	return GetDefaultAPIURL()
}

func GeneratePasscode() (string, error) {
	bytes := make([]byte, 24)

	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(bytes), nil
}
