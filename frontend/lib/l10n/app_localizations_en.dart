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
  String get projectDashboardChat => 'Chat';

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
  String get chatScreenSendButton => 'Send';

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
}
