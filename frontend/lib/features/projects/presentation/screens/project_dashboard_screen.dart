import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/api/api_exceptions.dart';
import 'package:frontend/core/widgets/data_load_error_message.dart';
import 'package:frontend/features/projects/data/project_providers.dart';
import 'package:frontend/features/projects/presentation/controllers/project_settings_controller.dart';
import 'package:frontend/features/projects/presentation/widgets/project_dashboard_shell.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';

void _goProjectsList(BuildContext context) => context.go('/projects');

class ProjectDashboardScreen extends ConsumerStatefulWidget {
  const ProjectDashboardScreen({
    super.key,
    required this.projectId,
    required this.navigationShell,
  });

  final String projectId;
  final StatefulNavigationShell navigationShell;

  @override
  ConsumerState<ProjectDashboardScreen> createState() =>
      _ProjectDashboardScreenState();
}

class _ProjectDashboardScreenState
    extends ConsumerState<ProjectDashboardScreen> {
  // Шелл проекта — ЕДИНСТВЕННЫЙ авторитетный источник activeProjectId: он знает
  // id из pathParameters, один на все ветки и переживает их переключение. Разбор
  // URL в rootRouterRedirect ненадёжен — у дефолтной ветки StatefulShellRoute
  // (Дашборд = chat) matchedLocation/uri теряют :id (видно по хлебным крошкам:
  // на Дашборде «Проекты», на Задачах «Проекты > id > tasks»), из-за чего
  // activeProjectId обнулялся и ассистент уходил в глобальный scope.
  void _syncActiveProject() {
    final notifier = ref.read(activeProjectIdProvider.notifier);
    if (ref.read(activeProjectIdProvider) != widget.projectId) {
      notifier.set(widget.projectId);
    }
  }

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) _syncActiveProject();
    });
  }

  @override
  void didUpdateWidget(ProjectDashboardScreen oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.projectId != widget.projectId) {
      _syncActiveProject();
    }
  }

  @override
  void dispose() {
    // Уход из проекта (шелл размонтирован) → сбрасываем scope в глобальный.
    // Сбрасываем только если всё ещё «наш» id — иначе затёрли бы scope проекта,
    // в который пользователь уже перешёл. try/catch: при teardown контейнер
    // провайдеров может быть уничтожен раньше виджета.
    try {
      if (ref.read(activeProjectIdProvider) == widget.projectId) {
        ref.read(activeProjectIdProvider.notifier).set(null);
      }
    } catch (_) {
      // контейнер уже уничтожен — нечего сбрасывать
    }
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final projectId = widget.projectId;
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
      overlay = const Center(
        key: ValueKey('project-dashboard-loading'),
        child: CircularProgressIndicator(),
      );
    } else if (asyncProject.hasError) {
      overlay = DataLoadErrorMessage(
        key: const ValueKey('project-dashboard-error'),
        title: l10n.dataLoadError,
        actionLabel: l10n.retry,
        onAction: () => ref.invalidate(projectProvider(projectId)),
      );
    }

    return Scaffold(
      appBar: AppBar(
        leading: const _ProjectDashboardBackButton(),
        title: Text(title),
        actions: [
          _ReindexButton(projectId: projectId),
        ],
      ),
      body: ProjectDashboardShell(
        navigationShell: widget.navigationShell,
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
        final navigator = Navigator.of(context);
        if (navigator.canPop()) {
          navigator.pop();
        } else {
          context.go('/projects');
        }
      },
    );
  }
}

class _ReindexButton extends ConsumerStatefulWidget {
  const _ReindexButton({required this.projectId});

  final String projectId;

  @override
  ConsumerState<_ReindexButton> createState() => _ReindexButtonState();
}

class _ReindexButtonState extends ConsumerState<_ReindexButton> {
  bool _isLoading = false;

  Future<void> _handleReindex() async {
    if (_isLoading) {
      return;
    }
    setState(() {
      _isLoading = true;
    });

    final messenger = ScaffoldMessenger.of(context);
    final theme = Theme.of(context);
    final l10n = AppLocalizations.of(context)!;

    try {
      final repo = ref.read(projectRepositoryProvider);
      await repo.reindex(widget.projectId);
      ref.invalidate(projectProvider(widget.projectId));
      if (mounted) {
        messenger.showSnackBar(
          SnackBar(
            content: Text(l10n.projectSettingsReindexStarted),
            backgroundColor: theme.colorScheme.secondary,
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        final title = projectSettingsReindexErrorTitle(l10n, e);
        final detail = projectSettingsErrorDetail(e);
        final detailStyle = TextStyle(
          fontSize: 12,
          color: theme.colorScheme.onError.withValues(alpha: 0.8),
        );
        messenger.showSnackBar(
          SnackBar(
            content: detail != null
                ? Column(
                    mainAxisSize: MainAxisSize.min,
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(title),
                      Text(detail, style: detailStyle),
                    ],
                  )
                : Text(title),
            backgroundColor: theme.colorScheme.error,
          ),
        );
      }
    } finally {
      if (mounted) {
        setState(() {
          _isLoading = false;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    return IconButton(
      icon: _isLoading
          ? const SizedBox(
              width: 24,
              height: 24,
              child: CircularProgressIndicator(
                strokeWidth: 2,
                valueColor: AlwaysStoppedAnimation<Color>(Colors.white),
              ),
            )
          : const Icon(Icons.sync),
      tooltip: l10n.projectSettingsReindex,
      onPressed: _isLoading ? null : _handleReindex,
    );
  }
}
