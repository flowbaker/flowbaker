package initialization

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/flowbaker/flowbaker/internal/version"
	"github.com/flowbaker/flowbaker/pkg/clients/flowbaker"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/rs/zerolog/log"
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

func fetchAPIPublicKey(apiURL string) (string, error) {
	client := flowbaker.NewClient(
		flowbaker.WithBaseURL(apiURL),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.GetAPIPublicKey(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get API public key: %w", err)
	}

	return resp.PublicKey, nil
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

func RunFirstTimeSetup() (*SetupResult, error) {
	showWelcome()

	executorName, address, err := collectExecutorConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to collect executor configuration: %w", err)
	}

	keys, err := GenerateAllKeys()
	if err != nil {
		return nil, fmt.Errorf("failed to generate keys: %w", err)
	}

	apiURL := getAPIURL()

	fmt.Println("📡 Registering with Flowbaker...")
	verificationCode, err := RegisterExecutor(executorName, address, keys, apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to register executor: %w", err)
	}

	frontendURL := GetVerificationURL(apiURL)
	connectionURL := fmt.Sprintf("%s/executors/verify?code=%s", frontendURL, verificationCode)
	fmt.Println()
	fmt.Println("🔗 Connect your executor:")
	fmt.Println()
	fmt.Printf("   %s\n", connectionURL)
	fmt.Println()
	fmt.Println("⏳ Waiting for connection...")

	executorID, workspaceIDs, workspaceNames, err := WaitForVerification(executorName, verificationCode, keys, apiURL)
	if err != nil {
		partialConfig := &ExecutorConfig{
			ExecutorName:     executorName,
			Address:          address,
			Keys:             keys,
			APIBaseURL:       apiURL,
			SetupComplete:    false,
			VerificationCode: verificationCode,
		}
		SaveConfig(partialConfig)
		return nil, fmt.Errorf("verification failed: %w", err)
	}

	fmt.Println("🔑 Fetching API public key...")
	apiPublicKey, err := fetchAPIPublicKey(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch API public key: %w", err)
	}

	config := &ExecutorConfig{
		ExecutorID:    executorID,
		ExecutorName:  executorName,
		Address:       address,
		WorkspaceIDs:  workspaceIDs,
		Keys:          keys,
		APIBaseURL:    apiURL,
		APIPublicKey:  apiPublicKey,
		SetupComplete: true,
		LastConnected: time.Now(),
	}

	if err := SaveConfig(config); err != nil {
		return nil, fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Println()
	fmt.Printf("✅ Connected to %d workspace(s): %s\n", len(workspaceNames), strings.Join(workspaceNames, ", "))
	fmt.Println("💾 Configuration saved")

	return &SetupResult{
		ExecutorID:     executorID,
		ExecutorName:   executorName,
		WorkspaceIDs:   workspaceIDs,
		WorkspaceNames: workspaceNames,
	}, nil
}

func getAPIURL() string {
	if url := os.Getenv("FLOWBAKER_API_URL"); url != "" {
		return url
	}
	return GetDefaultAPIURL()
}

func ResumeSetup() (*SetupResult, error) {
	config, err := LoadConfig()
	if err != nil || config == nil {
		return nil, fmt.Errorf("no setup to resume")
	}

	if config.SetupComplete {
		return &SetupResult{
			ExecutorID:     config.ExecutorID,
			ExecutorName:   config.ExecutorName,
			WorkspaceIDs:   config.WorkspaceIDs,
			WorkspaceNames: []string{"Unknown"}, // We don't store workspace names in config, would need API call to fetch
		}, nil
	}

	fmt.Println("🔄 Resuming setup...")
	fmt.Println()

	frontendURL := GetVerificationURL(config.APIBaseURL)
	connectionURL := fmt.Sprintf("%s/executors/verify?code=%s", frontendURL, config.VerificationCode)
	fmt.Println("🔗 Connect your executor:")
	fmt.Println()
	fmt.Printf("   %s\n", connectionURL)
	fmt.Println()
	fmt.Println("⏳ Waiting for connection...")

	executorID, workspaceIDs, workspaceNames, err := WaitForVerification(config.ExecutorName, config.VerificationCode, config.Keys, config.APIBaseURL)
	if err != nil {
		return nil, fmt.Errorf("verification failed: %w", err)
	}

	config.ExecutorID = executorID
	config.WorkspaceIDs = workspaceIDs
	config.SetupComplete = true
	config.LastConnected = time.Now()

	if err := SaveConfig(config); err != nil {
		return nil, fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Println()
	fmt.Printf("✅ Connected to %d workspace(s): %s\n", len(workspaceNames), strings.Join(workspaceNames, ", "))

	return &SetupResult{
		ExecutorID:     executorID,
		ExecutorName:   config.ExecutorName,
		WorkspaceIDs:   workspaceIDs,
		WorkspaceNames: workspaceNames,
	}, nil
}

// AddWorkspace adds the current executor to a new workspace
func AddWorkspace() (*SetupResult, error) {
	config, err := LoadConfig()
	if err != nil || config == nil {
		return nil, fmt.Errorf("no executor configuration found. Run setup first")
	}

	if !config.SetupComplete {
		return nil, fmt.Errorf("executor setup is not complete. Run setup first")
	}

	fmt.Println("🔗 Adding executor to new workspace...")
	fmt.Println()

	// Start health check server during registration
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		startSetupHealthCheckServer(ctx)
	}()

	// Use existing executor info to create registration
	apiURL := config.APIBaseURL
	verificationCode, err := RegisterExecutor(config.ExecutorName, config.Address, config.Keys, apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to register executor for new workspace: %w", err)
	}

	frontendURL := GetVerificationURL(apiURL)
	connectionURL := fmt.Sprintf("%s/executors/verify?code=%s", frontendURL, verificationCode)
	fmt.Println("🔗 Connect executor to new workspace:")
	fmt.Println()
	fmt.Printf("   %s\n", connectionURL)
	fmt.Println()
	fmt.Println("⏳ Waiting for workspace assignment...")

	executorID, newWorkspaceIDs, newWorkspaceNames, err := WaitForVerification(config.ExecutorName, verificationCode, config.Keys, apiURL)
	if err != nil {
		return nil, fmt.Errorf("workspace assignment failed: %w", err)
	}

	// Merge new workspace IDs with existing ones
	existingWorkspaceIDs := make(map[string]bool)
	for _, id := range config.WorkspaceIDs {
		existingWorkspaceIDs[id] = true
	}

	allWorkspaceIDs := make([]string, 0, len(config.WorkspaceIDs)+len(newWorkspaceIDs))
	allWorkspaceNames := make([]string, 0, len(config.WorkspaceIDs)+len(newWorkspaceNames))

	// Add existing workspaces
	for _, id := range config.WorkspaceIDs {
		allWorkspaceIDs = append(allWorkspaceIDs, id)
		allWorkspaceNames = append(allWorkspaceNames, "Unknown") // We don't store names, would need API call
	}

	// Add new workspaces (avoid duplicates)
	for i, id := range newWorkspaceIDs {
		if !existingWorkspaceIDs[id] {
			allWorkspaceIDs = append(allWorkspaceIDs, id)
			allWorkspaceNames = append(allWorkspaceNames, newWorkspaceNames[i])
		}
	}

	// Update configuration
	config.WorkspaceIDs = allWorkspaceIDs
	config.LastConnected = time.Now()

	if err := SaveConfig(config); err != nil {
		return nil, fmt.Errorf("failed to save updated configuration: %w", err)
	}

	fmt.Println()
	fmt.Printf("✅ Added to new workspace(s): %s\n", strings.Join(newWorkspaceNames, ", "))
	fmt.Printf("📋 Total workspaces: %d\n", len(allWorkspaceIDs))
	fmt.Println("💾 Configuration updated")

	return &SetupResult{
		ExecutorID:     executorID,
		ExecutorName:   config.ExecutorName,
		WorkspaceIDs:   allWorkspaceIDs,
		WorkspaceNames: allWorkspaceNames,
	}, nil
}


// startSetupHealthCheckServer starts a minimal HTTP server for verification during setup
func startSetupHealthCheckServer(ctx context.Context) {
	app := fiber.New(fiber.Config{
		AppName: "flowbaker-executor-setup",
	})

	app.Use(cors.New())
	app.Use(logger.New())

	app.Get("/health", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"status":     "healthy",
			"service":    "flowbaker-executor",
			"version":    version.GetVersion(),
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
			"setup_mode": true,
		})
	})

	if err := app.Listen(":8081", fiber.ListenConfig{
		GracefulContext:       ctx,
		DisableStartupMessage: true,
	}); err != nil {
		log.Error().Err(err).Msg("Setup health check server failed to start")
	}
}
