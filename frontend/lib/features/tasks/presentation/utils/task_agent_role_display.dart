import 'package:frontend/l10n/app_localizations.dart';

/// Роль назначенного агента (`assigned_agent.role`) → l10n (12.5).
String taskAgentRoleLabel(AppLocalizations l10n, String role) {
  return switch (role) {
    'worker' => l10n.taskAgentRoleWorker,
    'supervisor' => l10n.taskAgentRoleSupervisor,
    'orchestrator' => l10n.taskAgentRoleOrchestrator,
    'planner' => l10n.taskAgentRolePlanner,
    'developer' => l10n.taskAgentRoleDeveloper,
    'reviewer' => l10n.taskAgentRoleReviewer,
    'tester' => l10n.taskAgentRoleTester,
    'devops' => l10n.taskAgentRoleDevops,
    _ => l10n.taskAgentRoleUnknown,
  };
}
