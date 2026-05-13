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
  String get dashboardAdminManagePrompts => 'Manage Prompts (Admin)';

  @override
  String get dashboardAdminManageWorkflows => 'Manage Workflows (Admin)';

  @override
  String get dashboardAdminViewLlmLogs => 'View LLM Logs (Admin)';

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
  String get globalSettingsScreenTitle => 'Global LLM settings';

  @override
  String get globalSettingsStubIntro =>
      'LLM provider keys (OpenAI, Anthropic, Gemini, etc.) for agents are configured on the server for now. Full editing will be available after the API ships.';

  @override
  String get globalSettingsBlockedByLabel => 'Backend task in repo:';

  @override
  String get globalSettingsStubApiKeysNote =>
      'Below: DevTeam application API keys (MCP). These are not LLM provider keys.';

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
  String get projectSettingsSectionGit => 'Git repository';

  @override
  String get projectSettingsSectionVector => 'Vector index';

  @override
  String get projectSettingsSectionTechStack => 'Tech stack';

  @override
  String get projectSettingsGitDefaultBranchLabel => 'Default branch';

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
  String get taskDetailSectionSubtasks => 'Subtasks';

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
  String get globalSettingsTabDevTeam => 'DevTeam';

  @override
  String get llmProvidersSectionTitle => 'LLM providers';

  @override
  String get llmProvidersAdd => 'Add';

  @override
  String get llmProvidersEmpty => 'No LLM providers configured yet.';

  @override
  String get llmProvidersLoadError => 'Failed to load LLM providers';

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
  String get llmProvidersFieldUseProxy => 'Route via free-claude-proxy';

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
      'Open the link below in any browser and enter this code to authorize DevTeam:';

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
  String get agentSandboxSettingsCodeBackendLabel => 'Code backend';

  @override
  String get agentSandboxSettingsMCPHelper =>
      'JSON array of MCP server bindings: see docs.';

  @override
  String get agentSandboxSettingsSkillsHelper =>
      'JSON array of Claude Code skill refs: see docs.';

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
  String get agentSandboxRevokeConfirmTitle =>
      'Revoke Claude Code subscription?';

  @override
  String get agentSandboxRevokeConfirmBody =>
      'Agents will fall back to ANTHROPIC_API_KEY (if configured) for sandbox sessions. You can re-connect anytime.';

  @override
  String get teamAgentEditAdvanced => 'Advanced';

  @override
  String get commonRequestFailed => 'Request failed';
}
