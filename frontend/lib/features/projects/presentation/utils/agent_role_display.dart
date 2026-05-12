import 'package:frontend/l10n/app_localizations.dart';

/// Роль агента (`AgentModel.role`, `assigned_agent.role`) → l10n (DRY: 12.5, 13.1).
String agentRoleLabel(AppLocalizations l10n, String role) {
  return switch (role) {
    'worker' => l10n.agentRoleWorker,
    'supervisor' => l10n.agentRoleSupervisor,
    'orchestrator' => l10n.agentRoleOrchestrator,
    'planner' => l10n.agentRolePlanner,
    'developer' => l10n.agentRoleDeveloper,
    'reviewer' => l10n.agentRoleReviewer,
    'tester' => l10n.agentRoleTester,
    'devops' => l10n.agentRoleDevops,
    _ => l10n.agentRoleUnknown,
  };
}
