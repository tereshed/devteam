import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/routing/project_dashboard_routes.dart';

import '../../features/projects/helpers/project_dashboard_test_router.dart';

void main() {
  test(
    'projectDashboardShellBranchPaths — длина совпадает с buildProjectDashboardShellBranches',
    () {
      final branches = buildProjectDashboardShellBranches(
        chatNavigatorKey: kTestShellChatKey,
        tasksNavigatorKey: kTestShellTasksKey,
        teamNavigatorKey: kTestShellTeamKey,
        settingsNavigatorKey: kTestShellSettingsKey,
      );
      expect(branches.length, projectDashboardShellBranchPaths.length);
    },
  );
}
