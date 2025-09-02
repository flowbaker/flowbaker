package initialization

import (
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
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

func collectExecutorConfig() (string, string, error) {
	var executorName, address string

	// Get default values from environment or fallbacks
	defaultName := os.Getenv("FLOWBAKER_EXECUTOR_NAME")
	if defaultName == "" {
		defaultName = GenerateExecutorName()
	}

	defaultAddress := os.Getenv("FLOWBAKER_EXECUTOR_ADDRESS")
	if defaultAddress == "" {
		defaultAddress = "localhost:8081"
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
				Description("Address where your executor will listen").
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

	executorID, workspaceID, workspaceName, err := WaitForVerification(executorName, verificationCode, keys, apiURL)
	if err != nil {
		partialConfig := &ExecutorConfig{
			ExecutorName:     executorName,
			Keys:             keys,
			APIBaseURL:       apiURL,
			SetupComplete:    false,
			VerificationCode: verificationCode,
		}
		SaveConfig(partialConfig)
		return nil, fmt.Errorf("verification failed: %w", err)
	}

	config := &ExecutorConfig{
		ExecutorID:    executorID,
		ExecutorName:  executorName,
		WorkspaceID:   workspaceID,
		Keys:          keys,
		APIBaseURL:    apiURL,
		SetupComplete: true,
		LastConnected: time.Now(),
	}

	if err := SaveConfig(config); err != nil {
		return nil, fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Println()
	fmt.Printf("✅ Connected to workspace \"%s\"\n", workspaceName)
	fmt.Println("💾 Configuration saved")

	return &SetupResult{
		ExecutorID:    executorID,
		ExecutorName:  executorName,
		WorkspaceID:   workspaceID,
		WorkspaceName: workspaceName,
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
			ExecutorID:    config.ExecutorID,
			ExecutorName:  config.ExecutorName,
			WorkspaceID:   config.WorkspaceID,
			WorkspaceName: "Unknown", // We don't store workspace name in config, would need API call to fetch
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

	executorID, workspaceID, workspaceName, err := WaitForVerification(config.ExecutorName, config.VerificationCode, config.Keys, config.APIBaseURL)
	if err != nil {
		return nil, fmt.Errorf("verification failed: %w", err)
	}

	config.ExecutorID = executorID
	config.WorkspaceID = workspaceID
	config.SetupComplete = true
	config.LastConnected = time.Now()

	if err := SaveConfig(config); err != nil {
		return nil, fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Println()
	fmt.Printf("✅ Connected to workspace \"%s\"\n", workspaceName)

	return &SetupResult{
		ExecutorID:    executorID,
		ExecutorName:  config.ExecutorName,
		WorkspaceID:   workspaceID,
		WorkspaceName: workspaceName,
	}, nil
}
