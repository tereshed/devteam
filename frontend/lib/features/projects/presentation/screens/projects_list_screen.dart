import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/features/projects/data/project_providers.dart';
import 'package:frontend/features/projects/domain/models.dart';
import 'package:frontend/core/api/api_exceptions.dart';
import 'package:frontend/features/projects/domain/project_exceptions.dart';
import 'package:frontend/features/projects/domain/requests.dart';
import 'package:frontend/features/projects/presentation/utils/project_status_display.dart';
import 'package:frontend/features/projects/presentation/widgets/project_card.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';

/// Дебаунс поля поиска на списке проектов. Публичная константа — единый источник
/// правды для виджет-тестов (см. `docs/tasks/10.9-projects-widget-tests.md`).
const Duration kProjectsListSearchDebounce = Duration(milliseconds: 400);

class ProjectsListScreen extends ConsumerStatefulWidget {
  const ProjectsListScreen({super.key});

  @override
  ConsumerState<ProjectsListScreen> createState() => _ProjectsListScreenState();
}

class _ProjectsListScreenState extends ConsumerState<ProjectsListScreen> {
  static const _kLimit = 50;
  static const Duration _kRefreshTimeout = Duration(seconds: 30);

  final _searchController = TextEditingController();
  Timer? _debounce;
  String _searchQuery = '';
  String? _activeStatusFilter;
  bool _showClearButton = false;
  ProjectListResponse? _lastSeenData;

  /// Один family-вызов для watch / listen / invalidate / read(.future).
  ProjectListProvider _projectList(ProjectListFilter? f) =>
      projectListProvider(filter: f, limit: _kLimit);

  @override
  void dispose() {
    _debounce?.cancel();
    _searchController.dispose();
    super.dispose();
  }

  void _onSearchChanged(String value) {
    setState(() {
      _showClearButton = value.isNotEmpty;
    });
    _debounce?.cancel();
    _debounce = Timer(kProjectsListSearchDebounce, () {
      if (mounted) {
        setState(() {
          _searchQuery = value.trim();
        });
      }
    });
  }

  void _onSearchClear() {
    _searchController.clear();
    _debounce?.cancel();
    setState(() {
      _showClearButton = false;
      _searchQuery = '';
    });
  }

  void _onStatusFilterChanged(String? status) {
    setState(() {
      _activeStatusFilter = status;
    });
  }

  void _clearAllFilters() {
    _onSearchClear();
    setState(() {
      _activeStatusFilter = null;
    });
  }

  ProjectListFilter? get _currentFilter {
    final hasSearch = _searchQuery.isNotEmpty;
    final hasStatus = _activeStatusFilter != null;
    if (!hasSearch && !hasStatus) {
      return null;
    }
    return ProjectListFilter(
      search: hasSearch ? _searchQuery : null,
      status: _activeStatusFilter,
    );
  }

  Widget _scrollable(Widget child) => CustomScrollView(
        physics: const AlwaysScrollableScrollPhysics(),
        slivers: [SliverFillRemaining(hasScrollBody: false, child: child)],
      );

  Widget _buildLoading() =>
      _scrollable(const Center(child: CircularProgressIndicator()));

  Widget _buildError(
    Object error,
    AppLocalizations l10n,
    ProjectListFilter? filter,
  ) =>
      _scrollable(
        _ErrorState(
          message: _errorMessage(l10n, error),
          retryLabel: l10n.retry,
          onRetry: () {
            if (mounted) {
              ref.invalidate(_projectList(filter));
            }
          },
        ),
      );

  Widget _buildData(
    ProjectListResponse response,
    ProjectListFilter? filter,
    AppLocalizations l10n,
  ) {
    if (response.projects.isEmpty) {
      return _scrollable(
        _EmptyState(
          hasFilter: filter != null,
          l10n: l10n,
          onClearFilters: filter != null ? _clearAllFilters : null,
          onCreateProject: () => context.push('/projects/new'),
        ),
      );
    }
    return _ProjectsContent(projects: response.projects);
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final filter = _currentFilter;
    final asyncProjects = ref.watch(_projectList(filter));

    ref.listen<AsyncValue<ProjectListResponse>>(
      _projectList(filter),
      (_, next) {
        if (next.hasValue) {
          _lastSeenData = next.value;
        }
        // SnackBar при ошибке refresh: AsyncError в R3 сохраняет value ⇒ hasValue.
        if (next.hasError && next.hasValue) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(
              content: Text(_errorMessage(l10n, next.error!)),
              action: SnackBarAction(
                label: l10n.retry,
                onPressed: () {
                  final f = _currentFilter;
                  ref.invalidate(_projectList(f));
                },
              ),
            ),
          );
        }
      },
    );

    final staleData = asyncProjects.value;
    final effectiveData = staleData ?? _lastSeenData;

    final isFirstLoad = asyncProjects.isLoading && effectiveData == null;
    final isFirstError = asyncProjects.hasError && effectiveData == null;

    if (isFirstError) {
      debugPrint(
        'projectList error runtimeType=${asyncProjects.error?.runtimeType}',
      );
      debugPrint('projectList stackTrace:\n${asyncProjects.stackTrace}');
    }

    return Scaffold(
      appBar: AppBar(
        title: Text(l10n.projectsTitle),
        actions: [
          IconButton(
            icon: const Icon(Icons.add),
            tooltip: l10n.createProject,
            onPressed: () => context.push('/projects/new'),
          ),
        ],
      ),
      body: Column(
        children: [
          _SearchBar(
            l10n: l10n,
            controller: _searchController,
            onChanged: _onSearchChanged,
            showClear: _showClearButton,
            onClear: _onSearchClear,
          ),
          _StatusFilterBar(
            l10n: l10n,
            activeFilter: _activeStatusFilter,
            onFilterChanged: _onStatusFilterChanged,
          ),
          Expanded(
            child: RefreshIndicator(
              onRefresh: () async {
                final currentFilter = _currentFilter;
                try {
                  await ref
                      .refresh(_projectList(currentFilter).future)
                      .timeout(_kRefreshTimeout);
                } on TimeoutException {
                  // SnackBar — через ref.listen, если есть стейл; иначе UI без падения.
                } on Exception {
                  // Ошибки домена/сети при refresh; не глотаем Error (StateError и т.д.).
                }
              },
              child: isFirstLoad
                  ? _buildLoading()
                  : isFirstError
                      ? _buildError(asyncProjects.error!, l10n, filter)
                      : _buildData(effectiveData!, filter, l10n),
            ),
          ),
        ],
      ),
    );
  }

  String _errorMessage(AppLocalizations l10n, Object error) {
    if (error is UnauthorizedException) {
      return l10n.errorUnauthorized;
    }
    if (error is ProjectForbiddenException) {
      return l10n.errorForbidden;
    }
    return l10n.errorLoadingProjects;
  }
}

class _ProjectsContent extends StatelessWidget {
  const _ProjectsContent({required this.projects});

  static const _kMobileBreakpointWidth = 600.0;
  static const _kGridMaxCrossAxisExtent = 380.0;
  static const _kGridMainAxisExtent = 200.0;
  static const _kContentPadding = 16.0;
  static const _kListSeparatorGap = 12.0;
  static const _kGridGap = 12.0;

  final List<ProjectModel> projects;

  @override
  Widget build(BuildContext context) {
    final width = MediaQuery.sizeOf(context).width;

    if (width < _kMobileBreakpointWidth) {
      return ListView.separated(
        padding: const EdgeInsets.all(_kContentPadding),
        itemCount: projects.length,
        separatorBuilder: (context, _) =>
            const SizedBox(height: _kListSeparatorGap),
        itemBuilder: (context, i) => ProjectCard(project: projects[i]),
      );
    }

    return GridView.builder(
      padding: const EdgeInsets.all(_kContentPadding),
      gridDelegate: const SliverGridDelegateWithMaxCrossAxisExtent(
        maxCrossAxisExtent: _kGridMaxCrossAxisExtent,
        mainAxisExtent: _kGridMainAxisExtent,
        crossAxisSpacing: _kGridGap,
        mainAxisSpacing: _kGridGap,
      ),
      itemCount: projects.length,
      itemBuilder: (context, i) => ProjectCard(project: projects[i]),
    );
  }
}

class _SearchBar extends StatelessWidget {
  const _SearchBar({
    required this.l10n,
    required this.controller,
    required this.onChanged,
    required this.showClear,
    required this.onClear,
  });

  final AppLocalizations l10n;
  final TextEditingController controller;
  final ValueChanged<String> onChanged;
  final bool showClear;
  final VoidCallback onClear;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 12, 16, 4),
      child: TextField(
        controller: controller,
        onChanged: onChanged,
        decoration: InputDecoration(
          hintText: l10n.searchProjectsHint,
          prefixIcon: const Icon(Icons.search),
          suffixIcon: showClear
              ? IconButton(
                  icon: const Icon(Icons.clear),
                  onPressed: onClear,
                )
              : null,
          border: const OutlineInputBorder(),
          isDense: true,
        ),
      ),
    );
  }
}

class _StatusFilterBar extends StatelessWidget {
  const _StatusFilterBar({
    required this.l10n,
    required this.activeFilter,
    required this.onFilterChanged,
  });

  final AppLocalizations l10n;
  final String? activeFilter;
  final ValueChanged<String?> onFilterChanged;

  @override
  Widget build(BuildContext context) {
    final items = [null, ...projectStatuses];

    return SizedBox(
      height: 48,
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
        itemCount: items.length,
        separatorBuilder: (context, _) => const SizedBox(width: 8),
        itemBuilder: (context, i) {
          final status = items[i];
          final label = status == null
              ? l10n.filterAll
              : projectStatusDisplay(context, status).label;
          return FilterChip(
            label: Text(label),
            selected: activeFilter == status,
            onSelected: (selected) =>
                onFilterChanged(selected ? status : null),
          );
        },
      ),
    );
  }
}

class _EmptyState extends StatelessWidget {
  const _EmptyState({
    required this.hasFilter,
    required this.l10n,
    required this.onCreateProject,
    this.onClearFilters,
  });

  final bool hasFilter;
  final AppLocalizations l10n;
  final VoidCallback onCreateProject;
  final VoidCallback? onClearFilters;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(
            hasFilter ? Icons.search_off : Icons.folder_open,
            size: 64,
            color: Theme.of(context).colorScheme.outline,
          ),
          const SizedBox(height: 16),
          Text(
            hasFilter ? l10n.noProjectsMatchFilter : l10n.noProjectsYet,
            style: Theme.of(context).textTheme.bodyLarge,
          ),
          const SizedBox(height: 16),
          if (hasFilter && onClearFilters != null)
            TextButton(
              onPressed: onClearFilters,
              child: Text(l10n.clearFilters),
            )
          else
            FilledButton.icon(
              icon: const Icon(Icons.add),
              label: Text(l10n.createProject),
              onPressed: onCreateProject,
            ),
        ],
      ),
    );
  }
}

class _ErrorState extends StatelessWidget {
  const _ErrorState({
    required this.message,
    required this.retryLabel,
    required this.onRetry,
  });

  final String message;
  final String retryLabel;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(
            Icons.error_outline,
            size: 48,
            color: Theme.of(context).colorScheme.error,
          ),
          const SizedBox(height: 12),
          Text(message, textAlign: TextAlign.center),
          const SizedBox(height: 16),
          FilledButton.icon(
            icon: const Icon(Icons.refresh),
            label: Text(retryLabel),
            onPressed: onRetry,
          ),
        ],
      ),
    );
  }
}
