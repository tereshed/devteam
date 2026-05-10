import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/api/api_exceptions.dart';
import 'package:frontend/core/widgets/data_load_error_message.dart';
import 'package:frontend/features/projects/data/project_providers.dart';
import 'package:frontend/features/projects/presentation/widgets/project_dashboard_shell.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';

void _goProjectsList(BuildContext context) => context.go('/projects');

/// Дашборд проекта: загрузка [ProjectModel], shell с четырьмя ветками (см. роутер).
class ProjectDashboardScreen extends ConsumerWidget {
  const ProjectDashboardScreen({
    super.key,
    required this.projectId,
    required this.navigationShell,
  });

  final String projectId;
  final StatefulNavigationShell navigationShell;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final asyncProject = ref.watch(projectProvider(projectId));

    if (asyncProject.hasError &&
        asyncProject.error is ProjectNotFoundException) {
      return Scaffold(
        appBar: AppBar(
          leading: IconButton(
            icon: const Icon(Icons.arrow_back),
            onPressed: () => _goProjectsList(context),
            tooltip: MaterialLocalizations.of(context).backButtonTooltip,
          ),
          title: Text(l10n.projectDashboardFallbackTitle),
        ),
        body: DataLoadErrorMessage(
          title: l10n.projectDashboardNotFoundTitle,
          actionLabel: l10n.projectDashboardNotFoundBackToList,
          onAction: () => _goProjectsList(context),
        ),
      );
    }

    final title = asyncProject.maybeWhen(
      data: (p) => p.name,
      orElse: () => l10n.projectDashboardFallbackTitle,
    );

    Widget? overlay;
    if (asyncProject.isLoading) {
      overlay = const Center(child: CircularProgressIndicator());
    } else if (asyncProject.hasError) {
      overlay = DataLoadErrorMessage(
        title: l10n.dataLoadError,
        actionLabel: l10n.retry,
        onAction: () => ref.invalidate(projectProvider(projectId)),
      );
    }

    return Scaffold(
      appBar: AppBar(
        leading: const _ProjectDashboardBackButton(),
        title: Text(title),
      ),
      body: ProjectDashboardShell(
        navigationShell: navigationShell,
        overlay: overlay,
      ),
    );
  }
}

class _ProjectDashboardBackButton extends StatelessWidget {
  const _ProjectDashboardBackButton();

  @override
  Widget build(BuildContext context) {
    return IconButton(
      icon: const Icon(Icons.arrow_back),
      tooltip: MaterialLocalizations.of(context).backButtonTooltip,
      onPressed: () {
        if (context.canPop()) {
          context.pop();
        } else {
          context.go('/projects');
        }
      },
    );
  }
}
