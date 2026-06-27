// ignore: unused_import
import 'package:intl/intl.dart' as intl;
import 'app_localizations.dart';

// ignore_for_file: type=lint

/// The translations for English (`en`).
class AppLocalizationsEn extends AppLocalizations {
  AppLocalizationsEn([String locale = 'en']) : super(locale);

  @override
  String get appTitle => 'Wibe Flutter Gin Template';

  @override
  String get appShellBrand => 'PolyMaths';

  @override
  String get navDashboard => 'Overview';

  @override
  String get navProjects => 'Projects';

  @override
  String get navAgents => 'Agents';

  @override
  String get navWorktrees => 'Worktrees';

  @override
  String get navIntegrationsLlm => 'LLM providers';

  @override
  String get navIntegrationsGit => 'Git providers';

  @override
  String get navPrompts => 'Prompts';

  @override
  String get navWorkflows => 'Workflows';

  @override
  String get navExecutions => 'Executions';

  @override
  String get navSettings => 'Settings';

  @override
  String get navProfile => 'Profile';

  @override
  String get navApiKeys => 'API keys';

  @override
  String get navGroupHome => 'Home';

  @override
  String get navGroupResources => 'Resources';

  @override
  String get navGroupIntegrations => 'Integrations';

  @override
  String get navGroupAdmin => 'Administration';

  @override
  String get navGroupSettings => 'Settings';

  @override
  String get navBreadcrumbHome => 'Home';

  @override
  String get navBreadcrumbNew => 'New';

  @override
  String get integrationStatusConnected => 'Connected';

  @override
  String get integrationStatusDisconnected => 'Not connected';

  @override
  String get integrationStatusError => 'Error';

  @override
  String get integrationStatusPending => 'Connecting…';

  @override
  String get integrationsLlmTitle => 'LLM providers';

  @override
  String get integrationsLlmComingSoon =>
      'Provider management ships in stage 2. Below is a preview of the catalogue.';

  @override
  String get integrationsGitTitle => 'Git providers';

  @override
  String get integrationsGitComingSoon =>
      'Connecting GitHub and GitLab ships in stage 3.';

  @override
  String get integrationsGitConnectCta => 'Connect';

  @override
  String get integrationsGitGithubSubtitle =>
      'Read repositories, push to PR branches';

  @override
  String get integrationsGitGitlabSubtitle => 'Cloud and self-hosted GitLab';

  @override
  String get integrationsGitStage3Subtitle =>
      'Connect GitHub and GitLab to push branches and open merge requests.';

  @override
  String get integrationsGitSectionConnected => 'Connected';

  @override
  String get integrationsGitSectionAvailable => 'Available';

  @override
  String get integrationsGitDisconnectCta => 'Disconnect';

  @override
  String get integrationsGitConnectSelfHostedCta => 'Connect self-hosted';

  @override
  String get integrationsGitEmptyAvailable =>
      'All supported providers are already connected.';

  @override
  String get integrationsGitEmptyConnected => 'No providers connected yet.';

  @override
  String get integrationsGitReasonUserCancelled =>
      'Authorization was declined. Try again.';

  @override
  String get integrationsGitReasonExpired =>
      'OAuth session expired. Start over.';

  @override
  String get integrationsGitReasonProviderUnreachable =>
      'Git provider is unreachable. Try again later.';

  @override
  String get integrationsGitReasonInvalidHost =>
      'Host is not allowed (private network, unsupported scheme, or malformed URL).';

  @override
  String get integrationsGitReasonOauthNotConfigured =>
      'This provider is not configured on the server.';

  @override
  String get integrationsGitReasonRemoteRevokeFailed =>
      'Connection removed locally, but the provider did not confirm revocation. Revoke the token in your account settings as well.';

  @override
  String get integrationsGitReasonPending =>
      'Waiting for confirmation in the browser…';

  @override
  String integrationsGitReasonUnknown(String reason) {
    return 'Connection failed: $reason';
  }

  @override
  String get integrationsGitRetry => 'Retry';

  @override
  String integrationsGitLoadFailed(String message) {
    return 'Failed to load integrations: $message';
  }

  @override
  String integrationsGitConnectedHost(String host) {
    return 'Host: $host';
  }

  @override
  String integrationsGitConnectedAccount(String login) {
    return 'Account: $login';
  }

  @override
  String integrationsGitBrowserOpenFailed(String url) {
    return 'Couldn\'t open the browser. Open this URL manually: $url';
  }

  @override
  String get integrationsGitlabHostDialogTitle => 'Connect self-hosted GitLab';

  @override
  String get integrationsGitlabHostFieldHost => 'GitLab host (https://…)';

  @override
  String get integrationsGitlabHostFieldClientId => 'Application ID';

  @override
  String get integrationsGitlabHostFieldClientSecret => 'Application Secret';

  @override
  String get integrationsGitlabHostFieldHostHint =>
      'Stored as-is. Must be https (or http in local dev).';

  @override
  String get integrationsGitlabHostFieldSecretHint =>
      'Stored encrypted with AES-256-GCM.';

  @override
  String get integrationsGitlabHostFieldScopes => 'Scopes';

  @override
  String get integrationsGitlabHostFieldScopesHint =>
      'Must match the scopes enabled in your OAuth Application. \'api\' covers everything; for granular apps use e.g. \'read_api read_repository write_repository\'.';

  @override
  String get integrationsGitlabHostValidationScopesRequired =>
      'Enter at least one scope';

  @override
  String get integrationsGitlabHostValidationHostRequired =>
      'Enter your GitLab host URL';

  @override
  String get integrationsGitlabHostValidationHostScheme =>
      'Host must start with https:// (or http:// for local dev)';

  @override
  String get integrationsGitlabHostValidationHostFormat =>
      'Host URL is malformed';

  @override
  String get integrationsGitlabHostValidationClientIdRequired =>
      'Enter the Application ID';

  @override
  String get integrationsGitlabHostValidationClientSecretRequired =>
      'Enter the Application Secret';

  @override
  String get integrationsGitlabHostInstructionsToggle =>
      'How to register an Application in my GitLab';

  @override
  String get integrationsGitlabHostInstructionsStep1 =>
      'Open https://<your-gitlab-host>/-/user_settings/applications.';

  @override
  String get integrationsGitlabHostInstructionsStep2 =>
      'Click ‘Add new application’.';

  @override
  String integrationsGitlabHostInstructionsStep3(String redirectUri) {
    return 'Name: PolyMaths. Redirect URI: $redirectUri.';
  }

  @override
  String get integrationsGitlabHostInstructionsStep4 =>
      'Mark Confidential. Scope: api (it covers cloning, pushing and merge requests).';

  @override
  String get integrationsGitlabHostInstructionsStep5 =>
      'Save, copy Application ID and Secret, paste them above.';

  @override
  String get integrationsGitlabHostSubmitCta => 'Connect';

  @override
  String get integrationsGitlabHostCancelCta => 'Cancel';

  @override
  String get integrationsComingSoonChip => 'Coming soon';

  @override
  String get llmProviderClaudeCode => 'Claude Code';

  @override
  String get llmProviderAntigravity => 'Antigravity';

  @override
  String get llmProviderAntigravityOAuth => 'Antigravity subscription';

  @override
  String get llmProviderAnthropic => 'Anthropic';

  @override
  String get llmProviderOpenAi => 'OpenAI';

  @override
  String get llmProviderOpenRouter => 'OpenRouter';

  @override
  String get llmProviderDeepSeek => 'DeepSeek';

  @override
  String get llmProviderZhipu => 'Zhipu';

  @override
  String get llmProviderHermes => 'Hermes';

  @override
  String get llmProviderClaudeCodeSubtitle =>
      'Anthropic subscription via OAuth';

  @override
  String get llmProviderAntigravitySubtitle => 'Direct Antigravity API key';

  @override
  String get llmProviderAntigravityOAuthSubtitle =>
      'Antigravity subscription via OAuth';

  @override
  String get llmProviderAnthropicSubtitle => 'Direct Anthropic API key';

  @override
  String get llmProviderOpenAiSubtitle => 'GPT-4, GPT-4o, o-series';

  @override
  String get llmProviderOpenRouterSubtitle => 'Multi-provider aggregator';

  @override
  String get llmProviderDeepSeekSubtitle => 'DeepSeek Chat and Coder';

  @override
  String get llmProviderZhipuSubtitle => 'GLM models';

  @override
  String get llmProviderHermesSubtitle =>
      'Direct Nous Portal / Hermes API connection';

  @override
  String get integrationsLlmStage2Subtitle =>
      'Manage API keys and OAuth subscriptions for code agents.';

  @override
  String get integrationsLlmSectionConnected => 'Connected';

  @override
  String get integrationsLlmSectionAvailable => 'Available';

  @override
  String get integrationsLlmConnectCta => 'Connect';

  @override
  String get integrationsLlmDisconnectCta => 'Disconnect';

  @override
  String get integrationsLlmReplaceCta => 'Replace key';

  @override
  String get integrationsLlmEmptyAvailable =>
      'All supported providers are already connected.';

  @override
  String get integrationsLlmReasonUserCancelled =>
      'Access was declined. Try again.';

  @override
  String get integrationsLlmReasonExpired => 'Session expired. Start over.';

  @override
  String get integrationsLlmReasonProviderUnreachable =>
      'Provider is unreachable. Try again later.';

  @override
  String integrationsLlmReasonUnknown(String reason) {
    return 'Connection failed: $reason';
  }

  @override
  String get integrationsLlmReasonPending =>
      'Waiting for confirmation in the browser…';

  @override
  String get integrationsLlmRetry => 'Retry';

  @override
  String integrationsLlmDialogApiKeyTitle(String provider) {
    return 'Connect $provider';
  }

  @override
  String get integrationsLlmDialogApiKeyField => 'API key';

  @override
  String get integrationsLlmDialogApiKeyHint =>
      'Stored encrypted with AES-256-GCM.';

  @override
  String get integrationsLlmClaudeCodeManualTitle => 'Enter Claude Code token';

  @override
  String get integrationsLlmClaudeCodeManualHint =>
      'Use this if Anthropic OAuth is not configured on the server or you already have a setup-token.';

  @override
  String get integrationsLlmClaudeCodeManualAccessField =>
      'Access token (sk-ant-oat01-...)';

  @override
  String get integrationsLlmClaudeCodeManualRefreshField =>
      'Refresh token (optional)';

  @override
  String get integrationsLlmClaudeCodeManualCta => 'Use existing token';

  @override
  String get integrationsLlmClaudeCodeManualAccessRequired =>
      'Access token is required';

  @override
  String get integrationsLlmAntigravityManualTitle => 'Enter Antigravity token';

  @override
  String get integrationsLlmAntigravityManualHint =>
      'Use this if Antigravity OAuth is not configured on the server or you already have a token.';

  @override
  String get integrationsLlmAntigravityManualAccessField => 'Access token';

  @override
  String get integrationsLlmAntigravityManualRefreshField =>
      'Refresh token (optional)';

  @override
  String get integrationsLlmAntigravityManualCta => 'Use existing token';

  @override
  String get integrationsLlmAntigravityManualAccessRequired =>
      'Access token is required';

  @override
  String get integrationsLlmDialogApiKeyRequired => 'Enter a non-empty API key';

  @override
  String get integrationsLlmDialogCancel => 'Cancel';

  @override
  String get integrationsLlmDialogSave => 'Save';

  @override
  String get integrationsLlmClaudeCodeOAuthTitle => 'Connect Claude Code';

  @override
  String get integrationsLlmClaudeCodeOAuthStep1 =>
      'Open Anthropic in your browser, enter the code below and authorize the app.';

  @override
  String get integrationsLlmClaudeCodeOpenBrowser => 'Open browser';

  @override
  String get integrationsLlmClaudeCodeOAuthCode => 'Code:';

  @override
  String get integrationsLlmClaudeCodeOAuthCopy => 'Copy code';

  @override
  String get integrationsLlmClaudeCodeOAuthWaiting =>
      'Waiting for confirmation… You can close this dialog and come back later — the status will update automatically.';

  @override
  String get integrationsLlmClaudeCodeOAuthTimeout =>
      'Authorization timed out after 20 minutes. Try again.';

  @override
  String get integrationsLlmAntigravityOAuthTitle => 'Connect Antigravity';

  @override
  String get integrationsLlmAntigravityOAuthStep1 =>
      'Open Antigravity in your browser, enter the code below and authorize the app.';

  @override
  String get integrationsLlmAntigravityOpenBrowser => 'Open browser';

  @override
  String get integrationsLlmAntigravityOAuthCode => 'Code:';

  @override
  String get integrationsLlmAntigravityOAuthCopy => 'Copy code';

  @override
  String get integrationsLlmAntigravityOAuthWaiting =>
      'Waiting for confirmation… You can close this dialog and come back later — the status will update automatically.';

  @override
  String get integrationsLlmAntigravityOAuthTimeout =>
      'Authorization timed out after 20 minutes. Try again.';

  @override
  String integrationsLlmLoadFailed(String message) {
    return 'Failed to load integrations: $message';
  }

  @override
  String dashboardWelcomeUser(String email) {
    return 'Welcome, $email';
  }

  @override
  String get dashboardWelcomeAnon => 'Welcome';

  @override
  String get dashboardHubSubtitle =>
      'Overview of your projects, agents and integrations.';

  @override
  String dashboardStatProjectsActive(int n) {
    String _temp0 = intl.Intl.pluralLogic(
      n,
      locale: localeName,
      other: '$n active',
      one: '1 active',
      zero: 'No active',
    );
    return '$_temp0';
  }

  @override
  String dashboardStatProjectsTotal(int n) {
    String _temp0 = intl.Intl.pluralLogic(
      n,
      locale: localeName,
      other: '$n projects in total',
      one: '1 project in total',
      zero: 'No projects in total',
    );
    return '$_temp0';
  }

  @override
  String dashboardStatAgentsTotal(int n) {
    String _temp0 = intl.Intl.pluralLogic(
      n,
      locale: localeName,
      other: '$n agents',
      one: '1 agent',
      zero: 'No agents',
    );
    return '$_temp0';
  }

  @override
  String dashboardStatLlmConnected(int n) {
    String _temp0 = intl.Intl.pluralLogic(
      n,
      locale: localeName,
      other: '$n connected',
      one: '1 connected',
      zero: 'Not connected',
    );
    return '$_temp0';
  }

  @override
  String dashboardStatGitConnected(int n) {
    String _temp0 = intl.Intl.pluralLogic(
      n,
      locale: localeName,
      other: '$n connected',
      one: '1 connected',
      zero: 'Not connected',
    );
    return '$_temp0';
  }

  @override
  String get dashboardStatManageCta => 'Manage';

  @override
  String get dashboardStatComingSoon => 'Available in upcoming stage';

  @override
  String get dashboardRecentTasksTitle => 'Recent tasks';

  @override
  String get dashboardRecentTasksEmptyTitle => 'No tasks yet';

  @override
  String get dashboardRecentTasksEmptySubtitle =>
      'Create a project and add tasks to see them here.';

  @override
  String get dashboardRecentTasksError => 'Failed to load recent tasks.';

  @override
  String get login => 'Login';

  @override
  String get logout => 'Logout';

  @override
  String get register => 'Register';

  @override
  String get email => 'Email';

  @override
  String get password => 'Password';

  @override
  String get emailHint => 'example@mail.com';

  @override
  String get enterEmail => 'Enter email';

  @override
  String get enterValidEmail => 'Enter a valid email';

  @override
  String get enterPassword => 'Enter password';

  @override
  String passwordTooShort(int minLength) {
    return 'Password must be at least $minLength characters';
  }

  @override
  String get passwordsDoNotMatch => 'Passwords do not match';

  @override
  String get passwordMinLength => 'Password must be at least 8 characters';

  @override
  String get confirmPasswordPlaceholder => 'Confirm password';

  @override
  String get noAccountRegister => 'Don\'t have an account? Register';

  @override
  String get haveAccountLogin => 'Already have an account? Login';

  @override
  String get welcomeBack => 'Welcome';

  @override
  String get loginTitle => 'Login';

  @override
  String get registerTitle => 'Register';

  @override
  String get createAccount => 'Create Account';

  @override
  String get dashboard => 'Dashboard';

  @override
  String get dashboardAdminManagePrompts => 'Manage Prompts (Admin)';

  @override
  String get dashboardAdminManageWorkflows => 'Manage Workflows (Admin)';

  @override
  String get dashboardAdminViewLlmLogs => 'View LLM Logs (Admin)';

  @override
  String get dashboardAdminAgentsV2 => 'Agents (v2)';

  @override
  String get dashboardAdminWorktrees => 'Worktrees (debug)';

  @override
  String get profile => 'Profile';

  @override
  String get userInfo => 'User Information';

  @override
  String get role => 'Role';

  @override
  String get emailVerified => 'Email Verified';

  @override
  String get yes => 'Yes';

  @override
  String get no => 'No';

  @override
  String get goToProfile => 'Go to Profile';

  @override
  String get information => 'Information';

  @override
  String get refreshData => 'Refresh Data';

  @override
  String get dataLoadError => 'Error loading data';

  @override
  String get retry => 'Retry';

  @override
  String get userNotAuthorized => 'User not authorized';

  @override
  String get logoutConfirmTitle => 'Logout';

  @override
  String get logoutConfirmMessage => 'Are you sure you want to logout?';

  @override
  String get cancel => 'Cancel';

  @override
  String logoutError(String error) {
    return 'Logout error: $error';
  }

  @override
  String get errorInvalidCredentials => 'Invalid email or password';

  @override
  String get errorUserNotFound => 'User not found';

  @override
  String get errorUserAlreadyExists => 'User already exists';

  @override
  String get errorAccessDenied => 'Access denied';

  @override
  String get errorNetwork =>
      'Network error. Please check your internet connection.';

  @override
  String get errorRequestCancelled => 'The request was cancelled.';

  @override
  String get errorServer => 'Server error. Please try again later.';

  @override
  String get errorExternalService => 'Could not reach the external service.';

  @override
  String get errorUnknown => 'An unknown error occurred.';

  @override
  String get routerNavigationError => 'We couldn\'t open this page.';

  @override
  String get landingTitle => 'Build Faster with Wibe';

  @override
  String get landingSubtitle =>
      'The ultimate Flutter + Gin template for your next big idea.\nProduction-ready, scalable, and beautiful.';

  @override
  String get startForFree => 'Start for Free';

  @override
  String get learnMore => 'Learn More';

  @override
  String get whyWibe => 'Why Wibe?';

  @override
  String get featurePerformanceTitle => 'High Performance';

  @override
  String get featurePerformanceDesc =>
      'Built with Go (Gin) and Flutter for maximum speed.';

  @override
  String get featureSecurityTitle => 'Secure by Default';

  @override
  String get featureSecurityDesc =>
      'JWT Auth, RBAC, and best security practices included.';

  @override
  String get featureCrossPlatformTitle => 'Cross Platform';

  @override
  String get featureCrossPlatformDesc =>
      'Works perfectly on Web, iOS, Android, and Desktop.';

  @override
  String get getStarted => 'Get Started';

  @override
  String get goToDashboard => 'Go to Dashboard';

  @override
  String get promptsTitle => 'Prompts Management';

  @override
  String get promptsList => 'Prompts List';

  @override
  String get createPrompt => 'Create Prompt';

  @override
  String get editPrompt => 'Edit Prompt';

  @override
  String get deletePrompt => 'Delete Prompt';

  @override
  String get deletePromptConfirmation =>
      'Are you sure you want to delete this prompt?';

  @override
  String get promptName => 'Name (Unique ID)';

  @override
  String get promptDescription => 'Description';

  @override
  String get promptTemplate => 'Template';

  @override
  String get promptJsonSchema => 'JSON Schema (Optional)';

  @override
  String get promptIsActive => 'Is Active';

  @override
  String get promptNameRequired => 'Name is required';

  @override
  String get promptTemplateRequired => 'Template is required';

  @override
  String get invalidJson => 'Invalid JSON format';

  @override
  String get save => 'Save';

  @override
  String get update => 'Update';

  @override
  String get create => 'Create';

  @override
  String get delete => 'Delete';

  @override
  String get managePrompts => 'Manage Prompts (Admin)';

  @override
  String get templatePlaceholderHelper => 'Use <.Variable> for placeholders';

  @override
  String get apiKeysTitle => 'API Keys';

  @override
  String get apiKeyDescription =>
      'API keys allow your applications to access the API without a password. Each key acts on your behalf.';

  @override
  String get apiKeyCreate => 'Create Key';

  @override
  String get apiKeyName => 'Key Name';

  @override
  String get apiKeyNameHint => 'e.g. My Script, CI/CD';

  @override
  String get apiKeyExpiry => 'Expiration';

  @override
  String get apiKeyNoExpiry => 'No expiration';

  @override
  String get apiKeyExpiry30Days => '30 days';

  @override
  String get apiKeyExpiry90Days => '90 days';

  @override
  String get apiKeyExpiry1Year => '1 year';

  @override
  String get apiKeyCreated => 'Key Created';

  @override
  String get apiKeyCreatedWarning =>
      'Copy the key now! It will not be shown again.';

  @override
  String get apiKeyCopy => 'Copy Key';

  @override
  String get apiKeyCopied => 'Key copied to clipboard';

  @override
  String get apiKeyUnderstood => 'I have saved the key';

  @override
  String get apiKeyRevoke => 'Revoke';

  @override
  String get apiKeyRevokeTitle => 'Revoke Key';

  @override
  String get apiKeyRevokeConfirm =>
      'The key will stop working. This action is irreversible. Continue?';

  @override
  String get apiKeyDeleteTitle => 'Delete Key';

  @override
  String get apiKeyDeleteConfirm =>
      'The key will be permanently deleted. Continue?';

  @override
  String get apiKeyExpired => 'Expired';

  @override
  String get apiKeyCreatedAt => 'Created';

  @override
  String get apiKeyExpiresAt => 'Expires';

  @override
  String get apiKeyLastUsed => 'Last used';

  @override
  String get apiKeyEmpty => 'No API keys';

  @override
  String get apiKeyEmptyHint =>
      'Create a key to use the API from your applications';

  @override
  String get apiKeysManage => 'API Keys';

  @override
  String get globalSettingsScreenTitle => 'Global LLM settings';

  @override
  String get globalSettingsStubIntro =>
      'LLM provider keys (OpenAI, Anthropic, Gemini, etc.) for agents are configured on the server for now. Full editing will be available after the API ships.';

  @override
  String get globalSettingsBlockedByLabel => 'Backend task in repo:';

  @override
  String get globalSettingsStubApiKeysNote =>
      'Below: PolyMaths application API keys (MCP). These are not LLM provider keys.';

  @override
  String get globalSettingsOpenDevTeamApiKeys => 'Application API keys';

  @override
  String get mcpConfigTitle => 'MCP Configuration';

  @override
  String get mcpConfigDescription =>
      'Use this configuration to connect your LLM client (Cursor, Claude Desktop, VS Code Copilot) to this server';

  @override
  String get mcpConfigCopy => 'Copy Config';

  @override
  String get mcpConfigCopied => 'Configuration copied to clipboard';

  @override
  String get mcpConfigInstructions => 'Instructions:';

  @override
  String get mcpConfigStep1 => '1. Copy the configuration below';

  @override
  String get mcpConfigStep2 => '2. Open your LLM client settings';

  @override
  String get mcpConfigStep3Cursor => '   - Cursor: .cursor/config.json';

  @override
  String get mcpConfigStep3Claude =>
      '   - Claude Desktop: ~/Library/Application Support/Claude/claude_desktop_config.json';

  @override
  String get mcpConfigStep4 =>
      '3. Paste the configuration and restart the client';

  @override
  String get mcpConfigLoadError => 'Failed to load MCP configuration';

  @override
  String get mcpConfigDisabled => 'MCP server is disabled';

  @override
  String get projectsTitle => 'Projects';

  @override
  String get createProject => 'Create project';

  @override
  String get searchProjectsHint => 'Search projects...';

  @override
  String get filterAll => 'All';

  @override
  String get statusActive => 'Active';

  @override
  String get statusPaused => 'Paused';

  @override
  String get statusArchived => 'Archived';

  @override
  String get statusIndexing => 'Indexing';

  @override
  String get statusIndexingFailed => 'Indexing failed';

  @override
  String get statusReady => 'Ready';

  @override
  String get statusUnknown => 'Unknown';

  @override
  String get noProjectsYet => 'No projects yet';

  @override
  String get noProjectsMatchFilter => 'No projects match your filter';

  @override
  String get clearFilters => 'Clear filters';

  @override
  String get errorLoadingProjects => 'Failed to load projects';

  @override
  String get errorUnauthorized => 'Session expired. Please sign in again';

  @override
  String get errorForbidden => 'No access to projects';

  @override
  String get gitProviderGithub => 'GitHub';

  @override
  String get gitProviderGitlab => 'GitLab';

  @override
  String get gitProviderBitbucket => 'Bitbucket';

  @override
  String get gitProviderLocal => 'Local';

  @override
  String get gitProviderUnknown => 'Git';

  @override
  String get createProjectScreenTitle => 'New project';

  @override
  String get projectNameFieldLabel => 'Name';

  @override
  String get projectNameFieldHint => 'My project';

  @override
  String get projectNameRequired => 'Enter a name';

  @override
  String projectNameMaxLength(int max) {
    return 'Name must be at most $max characters';
  }

  @override
  String get projectDescriptionFieldLabel => 'Description';

  @override
  String get projectDescriptionFieldHint => 'What is this project for?';

  @override
  String get gitUrlFieldLabel => 'Repository URL';

  @override
  String get gitUrlFieldHint => 'https://...';

  @override
  String get gitUrlRequiredForRemote => 'Enter a repository URL';

  @override
  String get gitUrlInvalid => 'Enter a valid http or https URL';

  @override
  String get gitProviderFieldLabel => 'Git provider';

  @override
  String get createProjectErrorConflict => 'This name is already in use';

  @override
  String get createProjectErrorGeneric => 'Could not create the project';

  @override
  String get projectDashboardFallbackTitle => 'Project';

  @override
  String get projectDashboardChat => 'Dashboard';

  @override
  String get projectDashboardTasks => 'Tasks';

  @override
  String get projectDashboardTeam => 'Team';

  @override
  String get projectDashboardSettings => 'Settings';

  @override
  String get projectDashboardNotFoundTitle => 'Project not found';

  @override
  String get projectDashboardNotFoundBackToList => 'Back to projects';

  @override
  String get projectSettingsSectionGit => 'Git repository';

  @override
  String get repositoriesSectionTitle => 'Repositories';

  @override
  String get repositoriesSectionSubtitle =>
      'Git repositories of the project. The decomposer routes subtasks per repository.';

  @override
  String get repositoriesAddButton => 'Add repository';

  @override
  String get repositoriesEmpty => 'No repositories yet';

  @override
  String get repositoryPrimaryBadge => 'primary';

  @override
  String get repositoryFieldSlug => 'Slug';

  @override
  String get repositoryFieldSlugHint => 'e.g. ui, core, infra';

  @override
  String get repositoryFieldDisplayName => 'Display name';

  @override
  String get repositoryFieldUrl => 'Git URL';

  @override
  String get repositoryFieldBranch => 'Default branch';

  @override
  String get repositoryFieldProvider => 'Git provider';

  @override
  String get repositoryFieldRole => 'Role (for the decomposer)';

  @override
  String get repositoryFieldRoleHint => 'e.g. Flutter UI, high-load Go backend';

  @override
  String get repositoryAddDialogTitle => 'Add repository';

  @override
  String get repositoryAddSubmit => 'Add';

  @override
  String get repositoryRemoveTooltip => 'Remove repository';

  @override
  String get repositoryRemoveConfirmTitle => 'Remove repository?';

  @override
  String repositoryRemoveConfirmBody(String slug) {
    return 'Repository \"$slug\" will be detached from the project.';
  }

  @override
  String get repositoryRemoveConfirmAction => 'Remove';

  @override
  String get repositoryLastIndexedLabel => 'Last indexed';

  @override
  String get gitAccountSectionTitle => 'Git account';

  @override
  String get gitAccountFieldLabel => 'Account';

  @override
  String get gitAccountHelper =>
      'Which connected account to use for cloning and pull requests.';

  @override
  String get gitAccountNoneHint =>
      'No connected accounts for this provider. Connect one in Git providers.';

  @override
  String get gitAccountDefaultOption => 'Default (first connected account)';

  @override
  String get integrationsGitConnectAnotherCta => 'Connect another account';

  @override
  String get integrationsGitAccountsSectionTitle => 'Connected accounts';

  @override
  String get integrationsGitDisconnectAccountTooltip =>
      'Disconnect this account';

  @override
  String get createProjectAccountLabel => 'Git account';

  @override
  String get createProjectAccountLocal => 'Local (no git)';

  @override
  String get createProjectAccountNoneHint =>
      'No connected accounts. Connect one in Git providers to pick repositories.';

  @override
  String get projectSettingsSectionVector => 'Vector index';

  @override
  String get projectSettingsSectionTechStack => 'Tech stack';

  @override
  String get projectSettingsGitDefaultBranchLabel => 'Default branch';

  @override
  String get projectSettingsBranchNamingTitle => 'Branch naming';

  @override
  String get projectSettingsBranchTemplateLabel => 'Branch name template';

  @override
  String get projectSettingsBranchTemplateHint =>
      'Placeholders: ticket, slug, short_id, id, date. Fallback like ticket|short_id. Empty = default branch.';

  @override
  String get projectSettingsBranchPatternLabel =>
      'Strict format (regex, optional)';

  @override
  String get projectSettingsBranchPatternHint =>
      'Validates manual branch overrides. Leave empty to derive it from the template.';

  @override
  String get projectSettingsBranchLockLabel => 'Lock manual branch override';

  @override
  String get projectSettingsBranchLockSubtitle =>
      'Branch name is only generated from the template';

  @override
  String get projectSettingsBranchPreviewLabel => 'Preview';

  @override
  String get projectSettingsMrTitleTitle => 'MR / PR title';

  @override
  String get projectSettingsMrTitleLabel => 'MR title template';

  @override
  String get projectSettingsMrTitleHint =>
      'Placeholders: title, ticket, slug, branch, repo, short_id, date. Empty = PolyMaths: title.';

  @override
  String get projectSettingsGitCredentialCardTitle => 'Linked Git credential';

  @override
  String get projectSettingsUnlinkCredential => 'Unlink credential';

  @override
  String get projectSettingsUnlinkPendingHint =>
      'Credential will be removed when you save.';

  @override
  String get projectSettingsVectorCollectionLabel => 'Weaviate collection name';

  @override
  String get projectSettingsVectorCollectionHint => 'e.g. ProjectCode';

  @override
  String get projectSettingsVectorCollectionInvalid =>
      'Use a capital Latin letter first, then letters, digits, or underscores.';

  @override
  String get projectSettingsVectorCollectionRenamed =>
      'The collection name changed. Run reindex so vectors are written to the new collection; the old collection is not migrated automatically.';

  @override
  String get projectSettingsReindex => 'Reindex';

  @override
  String get projectSettingsReindexInProgress => 'Indexing…';

  @override
  String get projectSettingsReindexUnavailable =>
      'Reindex is unavailable for local projects or when the repository URL is empty.';

  @override
  String get projectSettingsReindexStarted => 'Reindexing started';

  @override
  String get projectSettingsReindexConflict =>
      'Indexing is already running or another conflict occurred.';

  @override
  String get projectSettingsReindexGenericError => 'Could not start reindex';

  @override
  String get projectSettingsReindexValidationError =>
      'Reindex request was rejected';

  @override
  String get projectSettingsTechStackAddRow => 'Add row';

  @override
  String get projectSettingsTechStackClear => 'Clear tech stack';

  @override
  String get projectSettingsTechStackKeyLabel => 'Key';

  @override
  String get projectSettingsTechStackValueLabel => 'Value';

  @override
  String get projectSettingsSave => 'Save';

  @override
  String get projectSettingsSaved => 'Settings saved';

  @override
  String get projectSettingsTabGeneral => 'General';

  @override
  String get projectSettingsTabVariables => 'Variables (tech stack)';

  @override
  String get projectSettingsNoChanges => 'No changes to save';

  @override
  String get projectSettingsGitRemoteAccessFailed =>
      'Could not reach the Git remote (clone or validation failed).';

  @override
  String get projectSettingsActionForbidden =>
      'This action is not allowed for your account.';

  @override
  String get projectSettingsSaveConflict => 'Save failed due to a conflict.';

  @override
  String get projectSettingsSaveGenericError => 'Could not save settings';

  @override
  String get projectSettingsSaveValidationError =>
      'Invalid data — check the form and try again.';

  @override
  String get projectSettingsIndexingStatusLabel => 'Indexing status';

  @override
  String get projectSettingsLastIndexedCommitLabel => 'Last indexed commit';

  @override
  String get chatErrorGeneric => 'Could not load chat';

  @override
  String get chatErrorConversationNotFound => 'Conversation not found';

  @override
  String get chatErrorRateLimited =>
      'Too many requests. Please try again later';

  @override
  String get chatScreenAppBarFallbackTitle => 'Chat';

  @override
  String get chatScreenSelectConversationHint =>
      'Pick a conversation or open one via a direct link with its id.';

  @override
  String chatScreenMessageSemanticUser(String text) {
    return 'You: $text';
  }

  @override
  String chatScreenMessageSemanticAssistant(String text) {
    return 'Assistant: $text';
  }

  @override
  String chatScreenMessageSemanticSystem(String text) {
    return 'System: $text';
  }

  @override
  String get chatScreenInputHint => 'Message…';

  @override
  String get chatScreenLoadingOlder => 'Loading older messages…';

  @override
  String get chatScreenPendingSending => 'Sending…';

  @override
  String get chatScreenPendingRetry => 'Retry send';

  @override
  String get chatScreenNotFoundBack => 'Back to projects';

  @override
  String get chatInputHint => 'Message…';

  @override
  String get chatInputSendTooltip => 'Send';

  @override
  String get chatInputStopTooltip => 'Cancel sending';

  @override
  String get chatInputAttachTooltip => 'Attach file';

  @override
  String get chatInputAttachDisabledHint => 'Attachments unavailable';

  @override
  String get taskStatusPending => 'Pending';

  @override
  String get taskStatusPlanning => 'Planning';

  @override
  String get taskStatusInProgress => 'In progress';

  @override
  String get taskStatusReview => 'Review';

  @override
  String get taskStatusTesting => 'Testing';

  @override
  String get taskStatusChangesRequested => 'Changes requested';

  @override
  String get taskStatusCompleted => 'Completed';

  @override
  String get taskStatusFailed => 'Failed';

  @override
  String get taskStatusCancelled => 'Cancelled';

  @override
  String get taskStatusPaused => 'Paused';

  @override
  String get taskStatusUnknownStatus => 'Unknown status';

  @override
  String get taskStatusActive => 'In progress';

  @override
  String get taskStatusDone => 'Done';

  @override
  String get taskStatusNeedsHuman => 'Needs human';

  @override
  String get tasksSearchHint => 'Search tasks';

  @override
  String get tasksEmpty => 'No tasks yet';

  @override
  String get tasksEmptyFiltered => 'No tasks match the filters';

  @override
  String get tasksEmptyFilteredClear => 'Clear filters';

  @override
  String get taskPriorityCritical => 'Critical';

  @override
  String get taskPriorityHigh => 'High';

  @override
  String get taskPriorityMedium => 'Medium';

  @override
  String get taskPriorityLow => 'Low';

  @override
  String get taskPriorityUnknown => 'Unknown priority';

  @override
  String get taskCardUnassigned => 'Unassigned';

  @override
  String taskCardAgentLine(String name, String role) {
    return '$name · $role';
  }

  @override
  String taskCardUpdatedAt(String time) {
    return 'Updated: $time';
  }

  @override
  String taskStatusCardFallbackTitle(String shortId) {
    return 'Task $shortId';
  }

  @override
  String get taskCardAgentRoleWorker => 'Worker';

  @override
  String get taskCardAgentRoleSupervisor => 'Supervisor';

  @override
  String get taskCardAgentRoleOrchestrator => 'Orchestrator';

  @override
  String get taskCardAgentRolePlanner => 'Planner';

  @override
  String get taskCardAgentRoleDeveloper => 'Developer';

  @override
  String get taskCardAgentRoleReviewer => 'Reviewer';

  @override
  String get taskCardAgentRoleTester => 'Tester';

  @override
  String get taskCardAgentRoleDevops => 'DevOps';

  @override
  String get taskErrorGeneric => 'Something went wrong with tasks';

  @override
  String get taskListErrorProjectNotFound => 'Project not found';

  @override
  String get taskDetailErrorTaskNotFound => 'Task not found';

  @override
  String get taskSendMessageNoIdempotencyHint =>
      'Tapping Send again creates another message on the server (idempotency is a separate task).';

  @override
  String get taskDetailAppBarLoading => 'Loading…';

  @override
  String get taskDetailRefreshTimedOut =>
      'Refresh is taking too long. Try again.';

  @override
  String get taskDetailDeletedTitle => 'Task deleted';

  @override
  String get taskDetailDeletedBody =>
      'This task was deleted on the server. Open the task list to continue with other items.';

  @override
  String get taskDetailSectionDescription => 'Description';

  @override
  String get taskDetailSectionResult => 'Result';

  @override
  String get taskDetailSectionDiff => 'Diff';

  @override
  String get taskDetailSectionMessages => 'Message log';

  @override
  String get taskDetailSectionErrorMessage => 'Task error';

  @override
  String get taskDetailSectionOutcome => 'Outcome';

  @override
  String get taskDetailSectionSubtasks => 'Subtasks';

  @override
  String get taskDetailSectionSandboxLogs => 'Sandbox Logs (Realtime)';

  @override
  String get taskDetailSandboxLogsEmpty =>
      'No sandbox logs yet. They will stream here in real-time when the agent (Developer/Tester) starts running.';

  @override
  String get taskDetailSandboxLogsClear => 'Clear';

  @override
  String get taskDetailSandboxLogsCopy => 'Copy';

  @override
  String get taskDetailSandboxLogsCopied => 'Logs copied to clipboard';

  @override
  String get taskDetailNoDiff => 'No diff';

  @override
  String get taskDetailNoDescription => 'No description';

  @override
  String get taskDetailNoResult => 'No result yet';

  @override
  String get taskDetailNoMessages => 'No messages yet';

  @override
  String get taskDetailBackToList => 'Back to task list';

  @override
  String get taskDetailProjectMismatch => 'Task belongs to another project';

  @override
  String get taskDetailRealtimeMutationBlocked =>
      'Task updates are temporarily unavailable';

  @override
  String get taskDetailRealtimeSessionFailure => 'Realtime session issue';

  @override
  String get taskDetailRealtimeServiceFailure => 'Realtime service error';

  @override
  String get taskActionPause => 'Pause';

  @override
  String get taskActionResume => 'Resume';

  @override
  String get taskActionCancel => 'Cancel task';

  @override
  String get taskActionCancelConfirmTitle => 'Cancel this task?';

  @override
  String get taskActionCancelConfirmBody =>
      'The task will be marked as cancelled. This cannot be undone.';

  @override
  String get taskActionConfirm => 'Yes, cancel task';

  @override
  String get taskActionBlockedByRealtimeSnack =>
      'Can\'t change the task while updates are temporarily unavailable.';

  @override
  String get taskActionAlreadyTerminalSnack => 'Task already finished';

  @override
  String get taskMessageTypeUnknown => 'Unknown message type';

  @override
  String get taskSenderTypeUnknown => 'Unknown sender';

  @override
  String get taskMessageTypeInstruction => 'Instruction';

  @override
  String get taskMessageTypeResult => 'Result';

  @override
  String get taskMessageTypeQuestion => 'Question';

  @override
  String get taskMessageTypeFeedback => 'Feedback';

  @override
  String get taskMessageTypeError => 'Error';

  @override
  String get taskMessageTypeComment => 'Comment';

  @override
  String get taskMessageTypeSummary => 'Summary';

  @override
  String get taskSenderTypeUser => 'User';

  @override
  String get taskSenderTypeAgent => 'Agent';

  @override
  String get agentRoleUnknown => 'Unknown role';

  @override
  String get agentRoleWorker => 'Worker';

  @override
  String get agentRoleSupervisor => 'Supervisor';

  @override
  String get agentRoleOrchestrator => 'Orchestrator';

  @override
  String get agentRolePlanner => 'Planner';

  @override
  String get agentRoleDeveloper => 'Developer';

  @override
  String get agentRoleReviewer => 'Reviewer';

  @override
  String get agentRoleTester => 'Tester';

  @override
  String get agentRoleDevops => 'DevOps';

  @override
  String get agentRoleDecomposer => 'Decomposer';

  @override
  String get agentRoleMerger => 'Merger';

  @override
  String get agentRoleRouter => 'Router';

  @override
  String get agentRoleAssistant => 'Assistant';

  @override
  String get teamEmptyAgents => 'No agents in this team yet.';

  @override
  String get teamAgentModelUnset => 'Default model';

  @override
  String get teamAgentNameUnset => 'Unnamed agent';

  @override
  String get teamAgentActive => 'Active';

  @override
  String get teamAgentInactive => 'Inactive';

  @override
  String get teamAgentEditTitle => 'Edit agent';

  @override
  String get teamAgentEditFieldModel => 'LLM model';

  @override
  String get teamAgentEditFieldPrompt => 'Prompt';

  @override
  String get teamAgentEditFieldCodeBackend => 'Code backend';

  @override
  String get teamAgentEditFieldProviderKind => 'LLM provider';

  @override
  String get teamAgentEditFieldProviderKindHelp =>
      'Provider keys live in Settings → LLM credentials';

  @override
  String get teamAgentEditFieldActive => 'Active';

  @override
  String get teamAgentEditSave => 'Save';

  @override
  String get teamAgentEditCancel => 'Cancel';

  @override
  String get teamAgentEditDiscardTitle => 'Discard changes?';

  @override
  String get teamAgentEditDiscardBody => 'Your edits will be lost.';

  @override
  String get teamAgentEditSaveError => 'Could not save agent';

  @override
  String get teamAgentEditSaveForbidden =>
      'You do not have permission to save this agent';

  @override
  String get teamAgentEditConflictError =>
      'Update rejected (conflict). Try again.';

  @override
  String get teamAgentEditNoPrompts => 'No prompts available';

  @override
  String get teamAgentEditPromptsLoadError => 'Failed to load prompts';

  @override
  String get teamAgentEditPromptNone => 'No prompt';

  @override
  String get teamAgentEditPromptSystemDefaultHardcoded =>
      'System default (hardcoded)';

  @override
  String get teamAgentEditPromptSystemDefaultHardcodedHelp =>
      'Using system default prompt (hardcoded)';

  @override
  String get teamAgentEditUnset => 'Not set';

  @override
  String get teamAgentEditDiscardConfirm => 'Discard';

  @override
  String get teamAgentEditRefetchError => 'Saved, but failed to refresh team';

  @override
  String get teamAgentEditFieldTools => 'Tools';

  @override
  String get teamAgentEditToolsLoadError => 'Failed to load tools catalog';

  @override
  String get teamAgentEditToolsEmpty => 'No tools available in the catalog';

  @override
  String get teamAgentEditToolsNoneSelected => 'No tools selected';

  @override
  String get teamAgentEditToolsValidationError =>
      'Invalid tool selection. Review your choices and try again.';

  @override
  String get teamAgentEditToolsRetry => 'Retry';

  @override
  String teamAgentEditToolsListEntryLabel(String name, String category) {
    return '$name ($category)';
  }

  @override
  String get teamAgentEditTestRun => 'Test run';

  @override
  String get teamAgentEditTestRunSuccess => 'Test task successfully started';

  @override
  String get teamAgentEditTestRunError => 'Failed to start test task';

  @override
  String get chatMessageCopyCode => 'Copy code';

  @override
  String get chatMessageStreamingPlaceholder => 'Typing…';

  @override
  String get chatMessageImagePlaceholder => '[image]';

  @override
  String chatMessageMarkdownImageAlt(String alt) {
    return '[$alt]';
  }

  @override
  String get refresh => 'Refresh';

  @override
  String get copy => 'Copy';

  @override
  String get openInBrowser => 'Open in browser';

  @override
  String get fieldRequired => 'Required';

  @override
  String get globalSettingsTabLLMProviders => 'LLM providers';

  @override
  String get globalSettingsTabClaudeCode => 'Claude Code';

  @override
  String get globalSettingsTabDevTeam => 'PolyMaths';

  @override
  String get assistantScopeGlobal => 'Global chat';

  @override
  String assistantScopeProject(String name) {
    return 'Project: $name';
  }

  @override
  String get assistantPromptUserTabTitle => 'Assistant';

  @override
  String get assistantPromptUserHeading => 'Assistant prompt (user level)';

  @override
  String get assistantPromptUserHint =>
      'Base system prompt for your assistant. New projects inherit a copy at creation time — later edits here do not affect already-created projects.';

  @override
  String get assistantPromptProjectTabTitle => 'Assistant';

  @override
  String get assistantPromptProjectHeading =>
      'Assistant prompt (project level)';

  @override
  String get assistantPromptProjectHint =>
      'System prompt for the assistant in this project. This is an independent copy that overrides the user prompt. Empty field falls back to the user prompt.';

  @override
  String get assistantPromptInherited =>
      'This project has no own prompt yet — the user prompt is used. Save to create a project copy.';

  @override
  String get assistantPromptSave => 'Save prompt';

  @override
  String get assistantPromptReset => 'Reset to user prompt';

  @override
  String get assistantPromptSaved => 'Assistant prompt saved';

  @override
  String get assistantPromptSaveError => 'Failed to save assistant prompt';

  @override
  String get assistantPromptLoadError => 'Failed to load assistant prompt';

  @override
  String get llmProvidersSectionTitle => 'LLM providers';

  @override
  String get llmProvidersAdd => 'Add';

  @override
  String get llmProvidersEmpty => 'No LLM providers configured yet.';

  @override
  String get llmProvidersLoadError => 'Failed to load LLM providers';

  @override
  String get llmProvidersAdminRequired =>
      'Admin role required to manage LLM providers';

  @override
  String get llmProvidersHealthTooltip => 'Health check';

  @override
  String get llmProvidersEditTooltip => 'Edit';

  @override
  String get llmProvidersDeleteTooltip => 'Delete';

  @override
  String get llmProvidersHealthOK => 'Provider is healthy';

  @override
  String get llmProvidersHealthFail => 'Health check failed';

  @override
  String get llmProvidersDeleteTitle => 'Delete LLM provider?';

  @override
  String llmProvidersDeleteConfirm(String name) {
    return 'Delete provider \"$name\"? Agents pointing at it will be left without a provider.';
  }

  @override
  String get llmProvidersDeleteFail => 'Delete failed';

  @override
  String get llmProvidersAddTitle => 'Add LLM provider';

  @override
  String get llmProvidersEditTitle => 'Edit LLM provider';

  @override
  String get llmProvidersFieldName => 'Name';

  @override
  String get llmProvidersFieldKind => 'Kind';

  @override
  String get llmProvidersFieldBaseURL => 'Base URL (optional)';

  @override
  String get llmProvidersFieldCredential => 'API key / token';

  @override
  String get llmProvidersFieldCredentialOptional =>
      'API key / token (leave empty to keep current)';

  @override
  String get llmProvidersFieldDefaultModel => 'Default model';

  @override
  String get llmProvidersFieldEnabled => 'Enabled';

  @override
  String get llmProvidersTest => 'Test';

  @override
  String get llmProvidersTestOK => 'Test connection succeeded';

  @override
  String get llmProvidersTestFail => 'Test connection failed';

  @override
  String get claudeCodeAuthLoadError =>
      'Failed to load Claude Code subscription status';

  @override
  String get claudeCodeAuthConnectedTitle =>
      'Claude Code subscription connected';

  @override
  String get claudeCodeAuthTokenType => 'Token type';

  @override
  String get claudeCodeAuthScopes => 'Scopes';

  @override
  String get claudeCodeAuthExpiresAt => 'Expires at';

  @override
  String get claudeCodeAuthLastRefreshedAt => 'Last refreshed at';

  @override
  String get claudeCodeAuthRevoke => 'Revoke';

  @override
  String get claudeCodeAuthRevokeOK => 'Subscription revoked';

  @override
  String get claudeCodeAuthDisconnectedTitle => 'Claude Code subscription';

  @override
  String get claudeCodeAuthDisconnectedHint =>
      'Sign in with your Claude Code subscription to let agents authenticate via OAuth instead of a long-lived API key.';

  @override
  String get claudeCodeAuthLogin => 'Sign in with subscription';

  @override
  String get claudeCodeAuthDeviceFlowTitle => 'Authorize on Anthropic';

  @override
  String get claudeCodeAuthEnterCodeHint =>
      'Open the link below in any browser and enter this code to authorize PolyMaths:';

  @override
  String get claudeCodeAuthWaiting => 'Waiting for authorization…';

  @override
  String get agentSandboxSettingsTitle => 'Agent advanced settings';

  @override
  String get agentSandboxSettingsLoadError => 'Failed to load agent settings';

  @override
  String get agentSandboxSettingsTabProvider => 'Model / provider';

  @override
  String get agentSandboxSettingsTabMCP => 'MCP servers';

  @override
  String get agentSandboxSettingsTabSkills => 'Skills';

  @override
  String get agentSandboxSettingsTabPermissions => 'Permissions';

  @override
  String get agentSandboxSettingsProviderLabel => 'LLM provider';

  @override
  String get agentSandboxSettingsProviderNone => '— none —';

  @override
  String get agentSandboxSettingsAttachServicesLabel =>
      'Attach project test services';

  @override
  String get agentSandboxSettingsAttachServicesHelper =>
      'Bring up the project\'s ephemeral test services (e.g. PostgreSQL) for this agent\'s sandbox runs. Typically enabled for the tester.';

  @override
  String get agentSandboxSettingsCodeBackendLabel => 'Code backend';

  @override
  String get agentSandboxSettingsMCPHelper =>
      'JSON array of MCP servers. Inline server fields: name, type (sse/http/stdio), url, headers. In a header value you can reference a project variable via the secret:NAME prefix — it is injected at runtime and never written to the file.';

  @override
  String get agentSandboxSettingsSkillsHelper =>
      'JSON array of skills (Claude Code / Antigravity / Hermes). Fields: name, source (builtin/plugin/path; hermes: builtin/agentskills/path), config.files — a map of relative paths (SKILL.md required, plus scripts etc.) to file contents. Files are copied into the sandbox before start; scripts are run by the agent via bash/python.';

  @override
  String get agentSandboxSettingsDefaultMode => 'Default mode';

  @override
  String get agentSandboxSettingsAllow => 'Allow';

  @override
  String get agentSandboxSettingsDeny => 'Deny';

  @override
  String get agentSandboxSettingsAsk => 'Ask';

  @override
  String get agentSandboxSettingsJsonInvalid => 'JSON is invalid';

  @override
  String get agentSandboxSettingsPatternHint =>
      'Read | Edit | Bash(go test:*) | mcp__server';

  @override
  String get agentSandboxSettingsTabToolsets => 'Toolsets';

  @override
  String get agentSandboxSettingsHermesToolsetsLabel => 'Hermes toolsets';

  @override
  String get agentSandboxSettingsHermesToolsetsHelper =>
      'Pick which Hermes toolsets are exposed to the agent.';

  @override
  String get agentSandboxSettingsHermesPermLabel => 'Permission mode';

  @override
  String get agentSandboxSettingsHermesPermHelper =>
      'Only yolo and accept are allowed in headless sandbox.';

  @override
  String get agentSandboxSettingsHermesMaxTurnsLabel => 'Max turns';

  @override
  String get agentSandboxSettingsHermesTemperatureLabel =>
      'Temperature (optional)';

  @override
  String get agentSandboxRevokeConfirmTitle =>
      'Revoke Claude Code subscription?';

  @override
  String get agentSandboxRevokeConfirmBody =>
      'Agents will fall back to ANTHROPIC_API_KEY (if configured) for sandbox sessions. You can re-connect anytime.';

  @override
  String get teamAgentEditAdvanced => 'Advanced';

  @override
  String get commonRequestFailed => 'Request failed';

  @override
  String get commonRequiredField => 'Required';

  @override
  String get commonCancel => 'Cancel';

  @override
  String get commonSave => 'Save';

  @override
  String get commonCreate => 'Create';

  @override
  String get commonConfirm => 'Confirm';

  @override
  String get agentsV2Title => 'Agents (v2)';

  @override
  String get agentsV2Empty => 'No agents registered yet.';

  @override
  String get agentsV2Refresh => 'Refresh';

  @override
  String get agentsV2CreateButton => 'New agent';

  @override
  String get agentsV2CreateTitle => 'Create agent';

  @override
  String get agentsV2DetailTitle => 'Agent';

  @override
  String get agentsV2KindLlm => 'LLM';

  @override
  String get agentsV2KindSandbox => 'Sandbox';

  @override
  String get agentsV2FieldId => 'ID';

  @override
  String get agentsV2FieldName => 'Name';

  @override
  String get agentsV2FieldRole => 'Role';

  @override
  String get agentsV2FieldExecutionKind => 'Execution kind';

  @override
  String get agentsV2FieldRoleDescription =>
      'Role description (used in Router prompt)';

  @override
  String get agentsV2FieldSystemPrompt => 'System prompt';

  @override
  String get agentsV2FieldModel => 'Model';

  @override
  String get agentsV2FieldTemperature => 'Temperature';

  @override
  String get agentsV2FieldMaxTokens => 'Max tokens';

  @override
  String get agentsV2FieldCodeBackend => 'Code backend';

  @override
  String get agentsV2FieldIsActive => 'Active';

  @override
  String get agentsV2SectionConfig => 'Configuration';

  @override
  String get agentsV2AddSecretButton => 'Add / update secret';

  @override
  String get agentsV2SavedSnackbar => 'Agent saved.';

  @override
  String get agentsV2SecretSaved => 'Secret saved (encrypted).';

  @override
  String get agentsV2SecretDialogTitle => 'Set agent secret';

  @override
  String get agentsV2SecretKeyName => 'Key name';

  @override
  String get agentsV2SecretValue => 'Value';

  @override
  String get agentsV2SecretValueHelper =>
      'Stored AES-256-GCM encrypted. Read-back is not available — re-enter to rotate.';

  @override
  String get agentsV2SecretsHint =>
      'Secrets are stored encrypted server-side and never returned. Use the button above to set/rotate a secret value.';

  @override
  String get tasksCancelButton => 'Cancel task';

  @override
  String get tasksCancelConfirmTitle => 'Cancel task?';

  @override
  String get tasksCancelConfirmBody =>
      'All in-flight agents will be interrupted and the task moved to cancelled state.';

  @override
  String get tasksCancelInflightSuccess =>
      'Cancel requested. Agents will stop shortly.';

  @override
  String get tasksCustomTimeoutLabel => 'Custom timeout (e.g. 4h, 90m, 3600s)';

  @override
  String get tasksCustomTimeoutHelper =>
      'Overrides the default 4h orchestration timeout. Min 1m, max 72h.';

  @override
  String get tasksCustomTimeoutInvalid => 'Invalid duration. Use Nh / Nm / Ns.';

  @override
  String get tasksCustomTimeoutSectionTitle => 'Timeout';

  @override
  String get tasksCustomTimeoutNone => 'Default (4h)';

  @override
  String get tasksCustomTimeoutEdit => 'Edit';

  @override
  String get tasksExternalKeyTitle => 'Ticket key';

  @override
  String get tasksExternalKeyNone => 'none';

  @override
  String get tasksExternalKeyEdit => 'Edit ticket key';

  @override
  String get tasksExternalKeyLabel => 'Ticket key';

  @override
  String get tasksExternalKeyHelper =>
      'e.g. DEV-123. Letters, digits, dash and underscore, up to 64 chars.';

  @override
  String get tasksExternalKeyInvalid => 'Invalid ticket key format';

  @override
  String get tasksExternalKeySave => 'Save';

  @override
  String get tasksExternalKeySavedSnack => 'Ticket key saved';

  @override
  String get tasksExternalKeyClearedSnack => 'Ticket key cleared';

  @override
  String get tasksCustomTimeoutSave => 'Save';

  @override
  String get tasksCustomTimeoutClear => 'Reset to default';

  @override
  String get tasksCustomTimeoutClearDialogTitle => 'Reset timeout?';

  @override
  String get tasksCustomTimeoutClearDialogBody =>
      'The orchestrator will fall back to the global 4h default for this task.';

  @override
  String get tasksCustomTimeoutSavedSnack => 'Timeout updated.';

  @override
  String get tasksCustomTimeoutClearedSnack => 'Timeout reset to default.';

  @override
  String get worktreesTitle => 'Worktrees (debug)';

  @override
  String get worktreesEmpty => 'No active worktrees.';

  @override
  String get worktreesColTask => 'Task';

  @override
  String get worktreesColBranch => 'Branch';

  @override
  String get worktreesColState => 'State';

  @override
  String get worktreesColAllocated => 'Allocated';

  @override
  String get worktreesReleaseButton => 'Force release';

  @override
  String get worktreesReleasedSnackbar => 'Worktree released.';

  @override
  String get worktreesReleaseDialogTitle => 'Force release worktree?';

  @override
  String get worktreesReleaseDialogBody =>
      'git worktree remove --force will run right now. The agent (if any) will lose its working directory and uncommitted changes.';

  @override
  String get worktreesReleaseAlreadyReleased =>
      'Worktree was already released.';

  @override
  String get worktreesReleaseFailed => 'Failed to release worktree.';

  @override
  String get worktreesReleaseNotConfigured =>
      'Worktree manager is not configured on the server (WORKTREES_ROOT / REPO_ROOT unset). Ask an operator to enable the feature.';

  @override
  String get worktreesFilterAll => 'All';

  @override
  String get worktreesFilterAllocated => 'Allocated';

  @override
  String get worktreesFilterInUse => 'In use';

  @override
  String get worktreesFilterReleased => 'Released';

  @override
  String get routerTimelineSection => 'Router timeline';

  @override
  String get routerTimelineEmpty => 'No router decisions logged yet.';

  @override
  String get artifactsSection => 'Artifacts';

  @override
  String get artifactsEmpty => 'No artifacts produced yet.';

  @override
  String get artifactViewerOpen => 'Open full artifact';

  @override
  String artifactViewerTitle(String kind, String idShort) {
    return '$kind · $idShort';
  }

  @override
  String get artifactViewerClose => 'Close';

  @override
  String get artifactViewerCopyFull => 'Copy full content';

  @override
  String artifactViewerCopyFullForKind(String kind) {
    return 'Copy full $kind';
  }

  @override
  String artifactViewerCopiedSnack(int bytes) {
    return 'Copied $bytes bytes to clipboard.';
  }

  @override
  String get artifactViewerCopyFailedSnack => 'Failed to copy to clipboard.';

  @override
  String artifactViewerShowFull(int kb) {
    return 'Show full ($kb KB)';
  }

  @override
  String artifactViewerShowNext(int n) {
    return 'Show next $n';
  }

  @override
  String artifactViewerTruncatedNotice(int kb, int totalKb) {
    return 'Showing first $kb KB out of $totalKb KB.';
  }

  @override
  String get artifactViewerEmpty => 'No content stored for this artifact.';

  @override
  String artifactViewerLoadFailed(String error) {
    return 'Failed to load artifact: $error';
  }

  @override
  String get artifactViewerReviewDecision => 'Decision';

  @override
  String get artifactViewerReviewIssues => 'Issues';

  @override
  String get artifactViewerReviewSummary => 'Summary';

  @override
  String get artifactViewerReviewNoIssues => 'No issues reported.';

  @override
  String get artifactViewerTestPassed => 'Passed';

  @override
  String get artifactViewerTestFailed => 'Failed';

  @override
  String get artifactViewerTestSkipped => 'Skipped';

  @override
  String get artifactViewerTestDuration => 'Duration';

  @override
  String artifactViewerTestDurationMs(int ms) {
    return '$ms ms';
  }

  @override
  String artifactViewerTestFailuresHeader(int n) {
    return 'Failures ($n)';
  }

  @override
  String artifactViewerTestFailureFile(String file, int line) {
    return '$file:$line';
  }

  @override
  String get artifactViewerTestNoFailures => 'All checks green.';

  @override
  String get artifactViewerTestVerdict => 'Verdict';

  @override
  String get artifactViewerTestAcceptance => 'Acceptance criteria';

  @override
  String get artifactViewerTestChecks => 'Checks';

  @override
  String get artifactsNoSummary => '(no summary)';

  @override
  String get artifactViewerTestUnnamed => '(unnamed)';

  @override
  String artifactViewerFullTitle(String kind) {
    return '$kind · full';
  }

  @override
  String get assistantSidebarTitle => 'Assistant';

  @override
  String get assistantTabChat => 'Chat';

  @override
  String get assistantTabTasks => 'Tasks';

  @override
  String get assistantEmptyChat =>
      'Ask the assistant about your projects, tasks, or settings.';

  @override
  String get assistantInputHint => 'Message the assistant…';

  @override
  String get assistantSend => 'Send';

  @override
  String get assistantStop => 'Stop';

  @override
  String get assistantCopyMessage => 'Copy';

  @override
  String get assistantCopied => 'Copied';

  @override
  String get assistantConfirmTitle => 'Confirm action';

  @override
  String get assistantConfirmApprove => 'Approve';

  @override
  String get assistantConfirmDeny => 'Deny';

  @override
  String get assistantNoActiveTasks => 'No active tasks across your projects.';

  @override
  String get assistantActiveTaskInProgress => 'In progress';

  @override
  String get assistantToggleTooltip => 'Toggle assistant';

  @override
  String get assistantSessionBusy => 'Assistant is working…';

  @override
  String get assistantSessionStale =>
      'Session is unresponsive — retry shortly.';

  @override
  String assistantToolCallTitle(String tool) {
    return 'Tool $tool';
  }

  @override
  String get assistantToolResultStatusOk => 'OK';

  @override
  String get assistantToolResultStatusForbidden => 'Forbidden';

  @override
  String get assistantToolResultStatusError => 'Error';

  @override
  String get assistantToolResultStatusDenied => 'Denied';

  @override
  String get assistantToolResultStatusTruncated => 'Truncated';

  @override
  String get assistantToolResultStatusPending => 'Pending';

  @override
  String get assistantToolResultLabel => 'Result';

  @override
  String get assistantToolArgumentsLabel => 'Arguments';

  @override
  String get assistantNewSession => 'New chat';

  @override
  String get assistantSessionUntitled => 'Untitled chat';

  @override
  String get assistantOpenTask => 'Open';

  @override
  String get assistantLoadOlder => 'Load older messages';

  @override
  String get assistantRetry => 'Retry';

  @override
  String get assistantErrorGeneric => 'Something went wrong. Please try again.';

  @override
  String assistantConfirmSummaryFallback(String tool) {
    return 'The assistant wants to run $tool. Approve to proceed.';
  }

  @override
  String get assistantMessageRoleUser => 'You';

  @override
  String get assistantMessageRoleAssistant => 'Assistant';

  @override
  String get assistantMessageRoleSystem => 'System';

  @override
  String get assistantLockScreenMessage =>
      'The assistant is not configured. Please set up your LLM access keys to start.';

  @override
  String get assistantLockScreenButton => 'Go to key settings';

  @override
  String get assistantTaskStateActive => 'Active';

  @override
  String get assistantTaskStateDone => 'Done';

  @override
  String get assistantTaskStateFailed => 'Failed';

  @override
  String get assistantTaskStateCancelled => 'Cancelled';

  @override
  String get assistantTaskStateNeedsHuman => 'Needs human';

  @override
  String get assistantTaskStatePaused => 'Paused';

  @override
  String assistantStatusError(String error) {
    return 'Error loading status: $error';
  }

  @override
  String get assistantStatusAdminSetup =>
      'The assistant requires configuration by an administrator.';

  @override
  String get navMcpServers => 'MCP servers';

  @override
  String get navRolePrompts => 'Role prompts';

  @override
  String get agentConfigScreenTitle => 'Agent configuration';

  @override
  String get agentConfigSaveButton => 'Save';

  @override
  String get agentConfigLoadError => 'Failed to load agent configuration';

  @override
  String get agentConfigActiveLabel => 'Active';

  @override
  String get agentConfigActiveOn => 'Agent is active and can receive tasks';

  @override
  String get agentConfigActiveOff =>
      'Agent is inactive and will not receive tasks';

  @override
  String get agentConfigRoleSectionTitle => 'Role';

  @override
  String get agentConfigTypeSectionTitle => 'Execution type';

  @override
  String get agentConfigLLMSectionTitle => 'LLM settings';

  @override
  String get agentConfigMCPSectionTitle => 'MCP tools';

  @override
  String get agentConfigSkillsSectionTitle => 'Skills';

  @override
  String get agentConfigRoleLabel => 'Role';

  @override
  String get agentConfigRoleReadOnly => 'Auto-created role (read-only)';

  @override
  String get agentConfigTypeAPI => 'API (LLM)';

  @override
  String get agentConfigTypeSandbox => 'Sandbox';

  @override
  String get agentConfigProviderLabel => 'LLM provider';

  @override
  String get agentConfigModelLabel => 'Model';

  @override
  String get agentConfigModelHint => 'e.g. claude-sonnet-4-20250514';

  @override
  String get agentConfigTemperatureLabel => 'Temperature';

  @override
  String get agentConfigTemperatureDefault => 'default';

  @override
  String get agentConfigDevTeamMCP => 'PolyMaths MCP';

  @override
  String get agentConfigDevTeamMCPDesc =>
      'Built-in PolyMaths tools (task management, code search, etc.)';

  @override
  String get agentConfigExternalMCPTitle => 'External MCP servers';

  @override
  String get agentConfigNoExternalMCP => 'No external MCP servers configured';

  @override
  String get agentConfigAddMCPServer => 'Add MCP server';

  @override
  String get agentConfigNoSkills => 'No skills configured';

  @override
  String get agentConfigAddSkill => 'Add skill';

  @override
  String get agentConfigSaveSuccess => 'Agent configuration saved';

  @override
  String get agentConfigSaveError => 'Failed to save agent configuration';

  @override
  String get projectVariablesTitle => 'Project variables';

  @override
  String get projectVariablesHint =>
      'Secrets scoped to this project. Agents resolve them via dollar-sign placeholders.';

  @override
  String get projectVariablesLoadError => 'Failed to load project secrets';

  @override
  String get projectVariablesEmpty => 'No project secrets yet';

  @override
  String get projectVariablesAddButton => 'Add secret';

  @override
  String get projectVariablesEditTitle => 'Edit secret';

  @override
  String get projectVariablesAddTitle => 'Add secret';

  @override
  String get projectVariablesKeyLabel => 'Key name';

  @override
  String get projectVariablesKeyRequired => 'Key name is required';

  @override
  String get projectVariablesKeyInvalid =>
      'Must start with A-Z, then A-Z 0-9 _ (max 128)';

  @override
  String get projectVariablesValueLabel => 'Value';

  @override
  String get projectVariablesValueRequired => 'Value is required';

  @override
  String get projectVariablesInjectLabel => 'Inject into sandbox (env)';

  @override
  String get projectVariablesInjectHint =>
      'The value becomes an environment variable in the sandbox, and its name is advertised to the agent in the prompt.';

  @override
  String get projectVariablesDescriptionLabel => 'Description (optional)';

  @override
  String get projectVariablesEnvBadge => 'env';

  @override
  String get projectVariablesCancelButton => 'Cancel';

  @override
  String get projectVariablesSaveButton => 'Save';

  @override
  String get projectVariablesDeleteTitle => 'Delete secret';

  @override
  String get projectVariablesDeleteConfirm => 'Permanently delete secret';

  @override
  String get projectVariablesDeleteButton => 'Delete';

  @override
  String get userVariablesTitle => 'Personal variables';

  @override
  String get userVariablesHint =>
      'Secrets scoped to your account. Available to all agents running on your behalf.';

  @override
  String get userVariablesLoadError => 'Failed to load personal secrets';

  @override
  String get userVariablesEmpty => 'No personal secrets yet';

  @override
  String get userVariablesAddButton => 'Add secret';

  @override
  String get userVariablesAddTitle => 'Add personal secret';

  @override
  String get userVariablesKeyLabel => 'Key name';

  @override
  String get userVariablesKeyRequired => 'Key name is required';

  @override
  String get userVariablesKeyInvalid =>
      'Must start with A-Z, then A-Z 0-9 _ (max 128)';

  @override
  String get userVariablesValueLabel => 'Value';

  @override
  String get userVariablesValueRequired => 'Value is required';

  @override
  String get userVariablesCancelButton => 'Cancel';

  @override
  String get userVariablesSaveButton => 'Save';

  @override
  String get userVariablesDeleteTitle => 'Delete secret';

  @override
  String get userVariablesDeleteConfirm => 'Permanently delete secret';

  @override
  String get userVariablesDeleteButton => 'Delete';

  @override
  String get mcpRegistryScreenTitle => 'MCP servers registry';

  @override
  String get mcpRegistryRefreshTooltip => 'Refresh';

  @override
  String get mcpRegistryLoadError => 'Failed to load MCP servers';

  @override
  String get mcpRegistryEmpty => 'No MCP servers registered yet';

  @override
  String get mcpRegistryDeleteTitle => 'Delete MCP server';

  @override
  String get mcpRegistryDeleteConfirm => 'Deactivate MCP server';

  @override
  String get mcpRegistryCancelButton => 'Cancel';

  @override
  String get mcpRegistryDeleteButton => 'Delete';

  @override
  String get mcpRegistryAddTitle => 'Add MCP server';

  @override
  String get mcpRegistryEditTitle => 'Edit MCP server';

  @override
  String get mcpRegistryNameLabel => 'Name';

  @override
  String get mcpRegistryNameRequired => 'Name is required';

  @override
  String get mcpRegistryDescLabel => 'Description';

  @override
  String get mcpRegistryTransportLabel => 'Transport';

  @override
  String get mcpRegistryCommandLabel => 'Command';

  @override
  String get mcpRegistryURLLabel => 'URL';

  @override
  String get mcpRegistryScopeLabel => 'Scope';

  @override
  String get mcpRegistryActiveLabel => 'Active';

  @override
  String get mcpRegistrySaveButton => 'Save';

  @override
  String get rolePromptsScreenTitle => 'Agent role prompts';

  @override
  String get rolePromptsRefreshTooltip => 'Refresh';

  @override
  String get rolePromptsLoadError => 'Failed to load role prompts';

  @override
  String get rolePromptsEmpty => 'No role prompts configured yet';

  @override
  String get rolePromptsEditTitle => 'Edit role prompt';

  @override
  String get rolePromptsContentLabel => 'Prompt content';

  @override
  String get rolePromptsCancelButton => 'Cancel';

  @override
  String get rolePromptsSaveButton => 'Save';

  @override
  String get onboardingConnectLlmProvider =>
      'To get started, connect an LLM provider and select a model for your assistant. Without this, the assistant cannot process messages.';

  @override
  String get onboardingConfigureAssistant =>
      'Your assistant is almost ready! Select an LLM provider and model in the settings to start working.';

  @override
  String get onboardingGoToSettings => 'Go to settings';

  @override
  String get onboardingConfigureProjectAgents =>
      'Configure the router agent — select an LLM provider and model so that task orchestration can begin.';

  @override
  String get onboardingGoToTeam => 'Configure agents';

  @override
  String get chatInputVoiceTooltip => 'Voice input (Alt+V)';

  @override
  String get chatInputVoiceDisabledTooltip =>
      'Voice input is not active (configure the speech recognition model in assistant settings)';

  @override
  String chatInputVoiceRecordingHint(int seconds) {
    return 'Recording... Speak (${seconds}s). Press Alt+V to complete';
  }

  @override
  String get agentMatrixTitle => 'The Agent Matrix';

  @override
  String get agentMatrixTimelineTab => 'Timeline';

  @override
  String get agentMatrixGraphTab => 'Graph';

  @override
  String get agentMatrixStatusPending => 'Pending';

  @override
  String get agentMatrixStatusRunning => 'Running';

  @override
  String get agentMatrixStatusSuccess => 'Success';

  @override
  String get agentMatrixStatusFailed => 'Failed';

  @override
  String get taskVizTabTrace => 'Trace';

  @override
  String get taskVizTabFlow => 'Flow';

  @override
  String get taskTraceWaiting => 'Waiting for the first router decision…';

  @override
  String get taskTraceRouterLane => 'router';

  @override
  String get taskTraceLegendRouter => 'router decision';

  @override
  String get taskTraceLegendDependency => 'dependency';

  @override
  String get taskTraceChanges => 'Changes requested';

  @override
  String get projectKpiTotal => 'Total';

  @override
  String get projectKpiActive => 'In progress';

  @override
  String get projectKpiDone => 'Done';

  @override
  String get projectKpiAttention => 'Attention';

  @override
  String get projectKpiFailed => 'Failed';

  @override
  String get projectTaskFilterAll => 'All';

  @override
  String get projectTaskFilterIssues => 'Issues';

  @override
  String get projectOpenTask => 'Open task';

  @override
  String get tasksColStatus => 'Status';

  @override
  String get tasksColTask => 'Task';

  @override
  String get tasksColPriority => 'Priority';

  @override
  String get tasksColAgent => 'Agent';

  @override
  String get tasksColUpdated => 'Updated';

  @override
  String get teamAgentProviderNotConnected =>
      'Provider not connected — set it up in Integrations';

  @override
  String get teamAgentNoConfiguredProviders =>
      'No connected providers — set them up in Integrations';

  @override
  String get teamAgentBackendRequired => 'Choose a backend';

  @override
  String get teamAgentBackendNeedsProvider =>
      'Hermes requires a selected provider';

  @override
  String get teamAgentProviderBackendMismatch =>
      'Provider is not compatible with the selected backend';

  @override
  String get teamAgentBackendLlmDisabled => 'LLM role does not use a backend';

  @override
  String get teamAgentProviderNotConnectedShort => 'not connected';

  @override
  String get appShellNavCollapse => 'Collapse menu';

  @override
  String get appShellNavExpand => 'Expand menu';

  @override
  String get agentMatrixInspectorTitle => 'Agent Inspection';

  @override
  String get agentMatrixInspectorSubtasks => 'Subtasks';

  @override
  String get agentMatrixInspectorLogs => 'Logs';

  @override
  String get agentMatrixInspectorArtifacts => 'Artifacts';

  @override
  String get agentMatrixInspectorNoSubtasks =>
      'No subtasks executed by this agent yet.';

  @override
  String get agentMatrixInspectorNoArtifacts =>
      'No artifacts created by this agent yet.';

  @override
  String get agentMatrixInspectorSelectSubtask => 'Select Subtask';

  @override
  String get agentMatrixInspectorGeneralDiscussion => 'Discussion & Actions';

  @override
  String get webhooksTitle => 'Webhooks';

  @override
  String get webhooksEmpty => 'No webhooks configured';

  @override
  String get webhookCreate => 'Create webhook';

  @override
  String get webhookEdit => 'Edit webhook';

  @override
  String get webhookName => 'Name';

  @override
  String get webhookNameHint => 'my-service-webhook';

  @override
  String get webhookRouteTo => 'Route to';

  @override
  String get webhookRouteProject => 'Project Chat';

  @override
  String get webhookRouteTeam => 'Team Task';

  @override
  String get webhookSelectTeam => 'Select Team';

  @override
  String get webhookInstructions => 'Instructions (introductory message)';

  @override
  String get webhookInstructionsHint =>
      'Instructions for the agent on how to handle this webhook payload';

  @override
  String get webhookDescription => 'Description';

  @override
  String get webhookDescriptionHint => 'What is this webhook used for?';

  @override
  String get webhookUrl => 'Webhook URL';

  @override
  String get webhookSecret => 'Secret';

  @override
  String get webhookRegenerateSecret => 'Regenerate secret';

  @override
  String get webhookRequireSecret => 'Require signature (HMAC SHA-256)';

  @override
  String get webhookAllowedIps => 'Allowed IPs (comma separated)';

  @override
  String get webhookIsActive => 'Is active';

  @override
  String get webhookDeleteConfirm =>
      'Are you sure you want to delete this webhook?';

  @override
  String get webhookDelete => 'Delete';

  @override
  String get webhookSave => 'Save';

  @override
  String get webhookSaved => 'Webhook saved';

  @override
  String get webhookCreated => 'Webhook created';

  @override
  String get webhookRequiredName => 'Name is required';

  @override
  String get webhookTaskMappingTitle => 'Task Mapping';

  @override
  String get webhookTaskTitleTemplate => 'Title template';

  @override
  String get webhookTaskTitleTemplateHint => 'e.g. [Bug] <issue.title>';

  @override
  String get webhookTaskDescTemplate => 'Description template';

  @override
  String get webhookTaskDescTemplateHint =>
      'e.g. Reported by <user.name>\\n\\n<issue.body>';

  @override
  String get webhookTaskPriorityTemplate => 'Priority template';

  @override
  String get webhookTaskPriorityTemplateHint =>
      'e.g. <issue.priority> (Expected: low, medium, high, critical)';

  @override
  String get projectDashboardSchedules => 'Schedule';

  @override
  String get schedulesTitle => 'Scheduled tasks';

  @override
  String get schedulesEmpty => 'No scheduled tasks yet';

  @override
  String get schedulesAdd => 'New scheduled task';

  @override
  String get schedulesLoadError => 'Failed to load scheduled tasks';

  @override
  String get scheduleActive => 'Active';

  @override
  String get scheduleInactive => 'Disabled';

  @override
  String get scheduleNextRunLabel => 'Next run';

  @override
  String get scheduleLastRunLabel => 'Last run';

  @override
  String get scheduleNeverRun => 'not run yet';

  @override
  String get scheduleEnableTooltip => 'Enable';

  @override
  String get scheduleDisableTooltip => 'Disable';

  @override
  String get scheduleEdit => 'Edit';

  @override
  String get scheduleDelete => 'Delete';

  @override
  String get scheduleDeleteTitle => 'Delete schedule?';

  @override
  String get scheduleDeleteMessage =>
      'The schedule will be removed. Already created tasks remain.';

  @override
  String get scheduleCreateTitle => 'New scheduled task';

  @override
  String get scheduleEditTitle => 'Edit schedule';

  @override
  String get scheduleNameLabel => 'Name';

  @override
  String get scheduleNameHint => 'Nightly refactor';

  @override
  String get scheduleNameRequired => 'Enter a name';

  @override
  String get scheduleDescriptionLabel => 'Task description';

  @override
  String get scheduleDescriptionHint => 'What each task should do';

  @override
  String get scheduleTeamLabel => 'Team';

  @override
  String get scheduleTeamNone => 'No team';

  @override
  String get schedulePriorityLabel => 'Priority';

  @override
  String get scheduleFrequencyLabel => 'Frequency';

  @override
  String get scheduleFreqDaily => 'Daily';

  @override
  String get scheduleFreqWeekly => 'Weekly';

  @override
  String get scheduleFreqHourly => 'Every N hours';

  @override
  String get scheduleFreqCustom => 'Custom (cron)';

  @override
  String get scheduleTimeLabel => 'Time';

  @override
  String get scheduleIntervalHoursLabel => 'Interval, hours';

  @override
  String get scheduleWeekdaysLabel => 'Days of week';

  @override
  String get scheduleWeekdaysRequired => 'Select at least one day';

  @override
  String get scheduleCronLabel => 'Cron expression';

  @override
  String get scheduleCronHint => '0 9 * * 1-5';

  @override
  String get scheduleCronInvalid =>
      'Invalid cron expression (5 fields required)';

  @override
  String get scheduleCronPreviewLabel => 'Cron';

  @override
  String get scheduleSave => 'Save';

  @override
  String get scheduleCancel => 'Cancel';

  @override
  String get scheduleSavedSnack => 'Schedule saved';

  @override
  String get scheduleDeletedSnack => 'Schedule deleted';

  @override
  String get weekdayShortMon => 'Mon';

  @override
  String get weekdayShortTue => 'Tue';

  @override
  String get weekdayShortWed => 'Wed';

  @override
  String get weekdayShortThu => 'Thu';

  @override
  String get weekdayShortFri => 'Fri';

  @override
  String get weekdayShortSat => 'Sat';

  @override
  String get weekdayShortSun => 'Sun';

  @override
  String get sandboxServicesTabTitle => 'Test environment';

  @override
  String get sandboxServicesHeading => 'Ephemeral test services';

  @override
  String get sandboxServicesDescription =>
      'Declare disposable services (e.g. PostgreSQL) that are brought up next to the sandbox agent for DB-backed integration tests. An agent attaches them via its \'attach test services\' toggle (typically the tester). A throwaway password is generated per run — it is never stored.';

  @override
  String get sandboxServicesEmpty => 'No test services configured yet.';

  @override
  String get sandboxServicesAddButton => 'Add service';

  @override
  String get sandboxServicesLoadError => 'Failed to load test services.';

  @override
  String get sandboxServicesSavedSnack => 'Service saved';

  @override
  String get sandboxServicesDeletedSnack => 'Service deleted';

  @override
  String get sandboxServiceEnabledLabel => 'Enabled';

  @override
  String get sandboxServiceFormTitleNew => 'New test service';

  @override
  String get sandboxServiceFormTitleEdit => 'Edit test service';

  @override
  String get sandboxServiceAliasLabel => 'Alias (hostname, e.g. db)';

  @override
  String get sandboxServiceImageLabel => 'Image';

  @override
  String get sandboxServiceDbNameLabel => 'Database name';

  @override
  String get sandboxServiceDbUserLabel => 'Database user';

  @override
  String get sandboxServicePortLabel => 'Port';

  @override
  String get sandboxServiceReadyTimeoutLabel => 'Ready timeout (sec)';

  @override
  String get sandboxServiceSeedKindLabel => 'Seed';

  @override
  String get sandboxServiceSeedValueLabel => 'Seed value (repo path or SQL)';

  @override
  String get sandboxServiceSave => 'Save';

  @override
  String get sandboxServiceCancel => 'Cancel';

  @override
  String get sandboxServiceDelete => 'Delete';

  @override
  String get sandboxServiceDeleteTitle => 'Delete service?';

  @override
  String sandboxServiceDeleteConfirm(String alias) {
    return 'Delete the test service \"$alias\"?';
  }

  @override
  String get scoutTabTitle => 'Scout';

  @override
  String get scoutHeading => 'Project Scout';

  @override
  String get scoutDescription =>
      'When a user comes with a problem rather than a formalized task, the scout runs a headless sandbox pass on your subscription, reads the project repositories and gathers a context dossier (relevant files, how it works, approaches, open questions, proposed acceptance criteria) to help formulate the task. Available to the project assistant when enabled.';

  @override
  String get scoutEnabledLabel => 'Enabled';

  @override
  String get scoutEnabledHint =>
      'Lets the project assistant dispatch the scout for context gathering.';

  @override
  String get scoutBackendLabel => 'Backend';

  @override
  String get scoutBackendHint =>
      'Sandbox CLI. Dispatch currently supports claude-code (subscription).';

  @override
  String get scoutTimeoutLabel => 'Run timeout, seconds';

  @override
  String get scoutTimeoutHint =>
      '60–3600. Hard cap for a scout run in the sandbox.';

  @override
  String get scoutSubscriptionNote =>
      'The scout runs on the project owner\'s connected Claude subscription (not metered API). Without a connected subscription the run will fail.';

  @override
  String get scoutSaveButton => 'Save';

  @override
  String get scoutSavedSnack => 'Scout settings saved';

  @override
  String get scoutPromptHeading => 'Scout prompt';

  @override
  String get scoutPromptHint =>
      'Instructions for the scout. Empty — the built-in default prompt is used.';

  @override
  String get scoutPromptDefaultNotice => 'Using the built-in default prompt.';

  @override
  String get scoutRunsTitle => 'Runs';

  @override
  String get scoutRunsEmpty =>
      'No runs yet. Start a scout manually or let the assistant dispatch it.';

  @override
  String get scoutRunButton => 'Run scout';

  @override
  String get scoutRunStartedSnack =>
      'Scout started — the dossier will appear in the runs list';

  @override
  String get scoutRunDialogTitle => 'Run scout';

  @override
  String get scoutRunDialogHint =>
      'Describe the problem in your own words: what hurts and the desired outcome.';

  @override
  String get scoutRunDialogCancel => 'Cancel';

  @override
  String get scoutRunDialogStart => 'Run';

  @override
  String get scoutRunStatusRunning => 'Running';

  @override
  String get scoutRunStatusDone => 'Done';

  @override
  String get scoutRunStatusFailed => 'Failed';

  @override
  String get scoutDossierTitle => 'Dossier';

  @override
  String get scoutDossierEmpty => 'No dossier';

  @override
  String get scoutLoadError => 'Failed to load scout data';

  @override
  String get scoutProviderLabel => 'Provider';

  @override
  String get scoutProviderHint =>
      'Auth/provider. claude-code: anthropic_oauth = subscription. hermes: anthropic/openrouter/hermes.';

  @override
  String get scoutProviderNone => '— not set —';

  @override
  String get scoutProviderRequired =>
      'The hermes backend requires an explicit provider';

  @override
  String get scoutModelLabel => 'Model';

  @override
  String get scoutModelHint =>
      'e.g. claude-sonnet-4-6, anthropic/claude-3.5-sonnet. Empty — backend default.';

  @override
  String get scoutTemperatureLabel => 'Temperature';

  @override
  String get scoutAdvancedTitle => 'Advanced sandbox settings';

  @override
  String get scoutMcpLabel => 'MCP servers (JSON)';

  @override
  String get scoutMcpHint =>
      'JSON array of mcp_servers, same shape as the agent\'s. Empty — none.';

  @override
  String get scoutSkillsLabel => 'Skills (JSON)';

  @override
  String get scoutSkillsHint =>
      'JSON array of skills, same shape as the agent\'s. Empty — none.';

  @override
  String get scoutPermissionsLabel => 'Permissions (JSON)';

  @override
  String get scoutPermissionsHint =>
      'JSON object: allow/deny/ask/defaultMode (Claude Code). Empty — defaults.';

  @override
  String get scoutInvalidJsonSnack =>
      'Invalid JSON in advanced settings — fix and save again';

  @override
  String get enhancerTabTitle => 'Enhancement';

  @override
  String get enhancerHeading => 'Project Enhancer';

  @override
  String get enhancerDescription =>
      'A meta-agent analyzes task execution history (router loops, review cycles, feedback) and proposes targeted improvements: addenda to project agent prompts and project description edits. Every proposal goes through human review — nothing is applied automatically.';

  @override
  String get enhancerEnabledLabel => 'Enabled';

  @override
  String get enhancerAutonomyLabel => 'Apply mode';

  @override
  String get enhancerAutonomyPropose => 'Propose changes (human review)';

  @override
  String get enhancerAutonomyAutoApply => 'Apply automatically';

  @override
  String get enhancerAutonomyAutoApplySoon =>
      'Coming soon: auto-apply with effect measurement and rollback';

  @override
  String get enhancerCronLabel => 'Auto-run schedule (cron)';

  @override
  String get enhancerCronHint =>
      'E.g.: 0 9 * * 1 — every Monday at 9:00. Empty — manual runs only.';

  @override
  String get enhancerWindowLabel => 'Analysis window, days';

  @override
  String get enhancerMaxChangesLabel => 'Proposals limit per run';

  @override
  String get enhancerSaveButton => 'Save';

  @override
  String get enhancerSavedSnack => 'Enhancer settings saved';

  @override
  String get enhancerRunNowButton => 'Run analysis';

  @override
  String get enhancerRunStartedSnack =>
      'Analysis started — the report will appear in the runs list';

  @override
  String get enhancerRunInProgressSnack =>
      'A run is already in progress, please wait';

  @override
  String get enhancerRunsTitle => 'Runs';

  @override
  String get enhancerRunsEmpty =>
      'No runs yet. Start an analysis manually or set up a schedule.';

  @override
  String get enhancerRunStatusRunning => 'Running';

  @override
  String get enhancerRunStatusDone => 'Done';

  @override
  String get enhancerRunStatusFailed => 'Failed';

  @override
  String get enhancerTriggerManual => 'manual';

  @override
  String get enhancerTriggerCron => 'scheduled';

  @override
  String get enhancerReportTitle => 'Report';

  @override
  String get enhancerReportEmpty => 'Report is empty';

  @override
  String get enhancerChangesTitle => 'Proposed changes';

  @override
  String get enhancerChangesEmpty => 'No proposals in this run';

  @override
  String get enhancerChangeReasonLabel => 'Reasoning';

  @override
  String get enhancerChangeEffectLabel => 'Expected effect';

  @override
  String get enhancerChangePayloadLabel => 'Change';

  @override
  String get enhancerChangeStatusProposed => 'Proposed';

  @override
  String get enhancerChangeStatusApproved => 'Approved';

  @override
  String get enhancerChangeStatusApplied => 'Applied';

  @override
  String get enhancerChangeStatusRejected => 'Rejected';

  @override
  String get enhancerChangeStatusRolledBack => 'Rolled back';

  @override
  String get enhancerTargetAgentOverride => 'Agent prompt/settings';

  @override
  String get enhancerTargetProjectDescription => 'Project description';

  @override
  String get enhancerTargetProjectSettings => 'Project settings';

  @override
  String get enhancerLoadError => 'Failed to load enhancer data';

  @override
  String get enhancerChangeApplyButton => 'Apply';

  @override
  String get enhancerChangeRejectButton => 'Reject';

  @override
  String get enhancerChangeRollbackButton => 'Roll back';

  @override
  String get enhancerChangeAppliedSnack => 'Change applied';

  @override
  String get enhancerChangeRejectedSnack => 'Change rejected';

  @override
  String get enhancerChangeRolledBackSnack => 'Change rolled back';

  @override
  String get enhancerChangeConflictSnack =>
      'Failed: the target changed after the proposal was created — refresh the list and verify manually';

  @override
  String get repoEnvFilesTabTitle => 'Env files';

  @override
  String get repoEnvFilesHeading => 'Repository env file injection';

  @override
  String get repoEnvFilesDescription =>
      'Inject a file (e.g. .env) into a repository\'s working copy before the agent runs. The file is available to the agent and tests but is excluded from git (never committed or pushed). The content is stored encrypted.';

  @override
  String get repoEnvFilesSelectRepo => 'Repository';

  @override
  String get repoEnvFilesNoRepos =>
      'Add a repository to the project first (General tab).';

  @override
  String get repoEnvFilesNotConfigured =>
      'No env file configured for this repository yet.';

  @override
  String get repoEnvFilesEmpty => 'No env files for this repository yet.';

  @override
  String get repoEnvFilesAddButton => 'Add file';

  @override
  String get repoEnvFilesCreateTitle => 'New env file';

  @override
  String get repoEnvFilesEditTitle => 'Edit env file';

  @override
  String get repoEnvFilesConfiguredHidden =>
      'The content is hidden — saving overwrites the whole file.';

  @override
  String get repoEnvFilesUpdatedLabel => 'Updated:';

  @override
  String get repoEnvFilesFileNameLabel => 'File name';

  @override
  String get repoEnvFilesFileNameHint => 'e.g. .env';

  @override
  String get repoEnvFilesTargetDirLabel => 'Target folder (optional)';

  @override
  String get repoEnvFilesTargetDirHint =>
      'Relative path inside the repo; empty = root';

  @override
  String get repoEnvFilesContentLabel => 'File content';

  @override
  String get repoEnvFilesContentHint => 'KEY=value\nANOTHER=value';

  @override
  String get repoEnvFilesSave => 'Save';

  @override
  String get repoEnvFilesDelete => 'Delete';

  @override
  String get repoEnvFilesDeleteConfirm =>
      'Delete the env file for this repository?';

  @override
  String get repoEnvFilesSaved => 'Env file saved';

  @override
  String get repoEnvFilesDeleted => 'Env file deleted';

  @override
  String get repoEnvFilesLoadError => 'Failed to load the env file.';

  @override
  String get repoEnvFilesSaveError => 'Failed to save the env file.';

  @override
  String get repoEnvFilesValidationFileNameRequired => 'File name is required';

  @override
  String get repoEnvFilesValidationContentRequired => 'Content is required';

  @override
  String get assistantMcpTabTitle => 'MCP servers';

  @override
  String get assistantMcpHeading => 'Assistant MCP servers';

  @override
  String get assistantMcpDescription =>
      'External MCP servers (remote http/sse) — their tools become available to this project\'s assistant. Header values may reference project secrets (see the hint in the form).';

  @override
  String get assistantMcpEmpty => 'No MCP servers configured yet.';

  @override
  String get assistantMcpAddButton => 'Add server';

  @override
  String get assistantMcpLoadError => 'Failed to load MCP servers.';

  @override
  String get assistantMcpSavedSnack => 'Server saved';

  @override
  String get assistantMcpDeletedSnack => 'Server deleted';

  @override
  String get assistantMcpFormTitleNew => 'New MCP server';

  @override
  String get assistantMcpFormTitleEdit => 'Edit MCP server';

  @override
  String get assistantMcpNameLabel => 'Name';

  @override
  String get assistantMcpTransportLabel => 'Transport';

  @override
  String get assistantMcpUrlLabel => 'URL';

  @override
  String get assistantMcpHeadersLabel => 'Headers (one per line: Name: value)';

  @override
  String get assistantMcpHeadersHint =>
      'One header per line, e.g. \"Authorization: Bearer YOUR_TOKEN\". You can reference a project variable instead of a literal value; secrets are resolved server-side (Project variables tab).';

  @override
  String get assistantMcpRequireConfirmationLabel => 'Ask for confirmation';

  @override
  String get assistantMcpEnabledLabel => 'Enabled';

  @override
  String get assistantMcpSave => 'Save';

  @override
  String get assistantMcpCancel => 'Cancel';

  @override
  String get assistantMcpDelete => 'Delete';

  @override
  String get assistantMcpDeleteTitle => 'Delete server?';

  @override
  String assistantMcpDeleteConfirm(String name) {
    return 'Delete MCP server \"$name\"?';
  }
}
