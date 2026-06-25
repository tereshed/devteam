import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/features/integrations/git/data/git_integrations_providers.dart';
import 'package:frontend/features/integrations/git/data/git_repositories_provider.dart';
import 'package:frontend/features/integrations/git/domain/git_integration_model.dart';
import 'package:frontend/features/integrations/git/domain/git_repository_model.dart';
import 'package:frontend/features/projects/domain/models/project_model.dart';
import 'package:frontend/features/projects/presentation/utils/git_remote_url.dart';
import 'package:frontend/l10n/app_localizations.dart';

/// Источник git-репозитория, выбранный в [GitRepoSourceSelector].
class GitRepoSource {
  const GitRepoSource({
    required this.gitProvider,
    required this.accountId,
    required this.gitUrl,
    this.repo,
  });

  /// 'github' | 'gitlab' | 'local' — git_provider репо.
  final String gitProvider;

  /// git_integration_credential_id выбранного аккаунта (null — локально/вручную).
  final String? accountId;

  /// Clone URL выбранного/введённого репозитория ('' для локального без git).
  final String gitUrl;

  /// Метаданные выбранного репозитория (если выбран из списка/создан) — для
  /// автозаполнения slug/имени/ветки/описания. null при ручном вводе URL.
  final GitRepositoryModel? repo;
}

/// Селектор источника репозитория по аналогии с формой создания проекта:
/// выпадающий список подключённых git-аккаунтов (задаёт провайдера) → выпадающий
/// список репозиториев этого аккаунта с возможностью создать новый репозиторий
/// или ввести URL вручную.
///
/// Самодостаточен: watch'ит [gitAccountsProvider]/[gitRepositoriesProvider], сам
/// держит выбор. О каждом изменении сообщает через [onChanged]. URL-поля содержат
/// валидаторы, участвующие в объемлющей [Form].
class GitRepoSourceSelector extends ConsumerStatefulWidget {
  const GitRepoSourceSelector({
    super.key,
    required this.onChanged,
    this.enabled = true,
  });

  final ValueChanged<GitRepoSource> onChanged;
  final bool enabled;

  @override
  ConsumerState<GitRepoSourceSelector> createState() =>
      _GitRepoSourceSelectorState();
}

class _GitRepoSourceSelectorState extends ConsumerState<GitRepoSourceSelector> {
  final _urlCtrl = TextEditingController();

  String _gitProvider = kLocalGitProvider;
  String? _accountId;
  bool _manualUrl = false;
  GitRepositoryModel? _selectedRepo;

  bool get _isRemote => _gitProvider != kLocalGitProvider;

  GitIntegrationProvider? get _domainProvider =>
      GitIntegrationProvider.tryParse(_gitProvider);

  @override
  void dispose() {
    _urlCtrl.dispose();
    super.dispose();
  }

  void _notify() {
    widget.onChanged(
      GitRepoSource(
        gitProvider: _gitProvider,
        accountId: _accountId,
        gitUrl: _isRemote ? _urlCtrl.text.trim() : '',
        repo: _manualUrl ? null : _selectedRepo,
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final isBusy = !widget.enabled;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _buildAccountSelector(context, l10n, isBusy),
        if (_isRemote) ...[
          const SizedBox(height: 12),
          ..._buildRemoteGitUrlSection(context, l10n, isBusy),
        ],
      ],
    );
  }

  /// Дропдаун подключённых аккаунтов: выбор задаёт провайдера и accountId.
  Widget _buildAccountSelector(
    BuildContext context,
    AppLocalizations l10n,
    bool isBusy,
  ) {
    final theme = Theme.of(context);
    final accounts = ref.watch(gitAccountsProvider).maybeWhen(
          data: (list) => list
              .where((a) =>
                  a.status == GitProviderConnectionStatus.connected &&
                  a.id != null)
              .toList(),
          orElse: () => const <GitProviderConnection>[],
        );
    final hasSelected =
        _accountId != null && accounts.any((a) => a.id == _accountId);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        DropdownButtonFormField<String?>(
          // ignore: deprecated_member_use
          value: hasSelected ? _accountId : null,
          decoration: InputDecoration(
            labelText: l10n.createProjectAccountLabel,
          ),
          items: [
            DropdownMenuItem<String?>(
              value: null,
              child: Text(l10n.createProjectAccountLocal),
            ),
            for (final a in accounts)
              DropdownMenuItem<String?>(
                value: a.id,
                child: Text(_accountLabel(a)),
              ),
          ],
          onChanged: isBusy
              ? null
              : (id) {
                  setState(() {
                    _accountId = id;
                    if (id == null) {
                      _gitProvider = kLocalGitProvider;
                    } else {
                      _gitProvider = accounts
                          .firstWhere((a) => a.id == id)
                          .provider
                          .jsonValue;
                    }
                    _selectedRepo = null;
                    _manualUrl = false;
                    _urlCtrl.clear();
                  });
                  _notify();
                },
        ),
        if (accounts.isEmpty)
          Padding(
            padding: const EdgeInsets.only(top: 6),
            child: Text(
              l10n.createProjectAccountNoneHint,
              style: theme.textTheme.bodySmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
          ),
      ],
    );
  }

  String _accountLabel(GitProviderConnection a) {
    final provider =
        a.provider == GitIntegrationProvider.github ? 'GitHub' : 'GitLab';
    final login = a.accountLogin ?? '';
    final host = a.host ?? '';
    if (host.isNotEmpty) {
      return login.isNotEmpty
          ? '$provider · $login @ $host'
          : '$provider · $host';
    }
    return login.isNotEmpty ? '$provider · $login' : provider;
  }

  List<Widget> _buildRemoteGitUrlSection(
    BuildContext context,
    AppLocalizations l10n,
    bool isBusy,
  ) {
    final theme = Theme.of(context);
    final domainProvider = _domainProvider;

    // Провайдер без API (bitbucket) или ручной ввод — простое URL-поле.
    if (domainProvider == null || _manualUrl) {
      return [
        TextFormField(
          controller: _urlCtrl,
          decoration: InputDecoration(
            labelText: l10n.gitUrlFieldLabel,
            hintText: l10n.gitUrlFieldHint,
          ),
          keyboardType: TextInputType.url,
          textInputAction: TextInputAction.done,
          enabled: !isBusy,
          onChanged: (_) => _notify(),
          validator: (v) {
            final t = v?.trim() ?? '';
            if (t.isEmpty) {
              return l10n.gitUrlRequiredForRemote;
            }
            if (!isValidGitRemoteUrl(t)) {
              return l10n.gitUrlInvalid;
            }
            return null;
          },
        ),
        if (domainProvider != null) ...[
          const SizedBox(height: 8),
          Align(
            alignment: Alignment.centerLeft,
            child: TextButton.icon(
              onPressed: isBusy
                  ? null
                  : () => setState(() => _manualUrl = false),
              icon: const Icon(Icons.list, size: 16),
              label: const Text('Выбрать из списка моих репозиториев'),
            ),
          ),
        ],
      ];
    }

    // Список репозиториев аккаунта. Ключ включает _accountId — иначе при смене
    // аккаунта вернётся закэшированный список первого аккаунта провайдера.
    final reposArgs = (provider: domainProvider, accountId: _accountId);
    final reposAsync = ref.watch(gitRepositoriesProvider(reposArgs));
    return [
      reposAsync.when(
        data: (repos) {
          final items = List<GitRepositoryModel>.from(repos);
          if (_selectedRepo != null &&
              !items.any((r) => r.cloneUrl == _selectedRepo!.cloneUrl)) {
            items.insert(0, _selectedRepo!);
          }
          return Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              DropdownButtonFormField<GitRepositoryModel>(
                // ignore: deprecated_member_use
                value: _selectedRepo,
                isExpanded: true,
                decoration: const InputDecoration(
                  labelText: 'Репозиторий',
                  hintText: 'Выберите репозиторий',
                ),
                items: items
                    .map((repo) => DropdownMenuItem<GitRepositoryModel>(
                          value: repo,
                          child: Text(
                            repo.fullName,
                            overflow: TextOverflow.ellipsis,
                          ),
                        ))
                    .toList(),
                onChanged: isBusy
                    ? null
                    : (repo) {
                        setState(() {
                          _selectedRepo = repo;
                          if (repo != null) {
                            _urlCtrl.text = repo.cloneUrl;
                          }
                        });
                        _notify();
                      },
                validator: (v) {
                  if (v == null && !_manualUrl) {
                    return 'Пожалуйста, выберите репозиторий';
                  }
                  return null;
                },
              ),
              const SizedBox(height: 4),
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Expanded(
                    child: Wrap(
                      spacing: 8,
                      children: [
                        TextButton.icon(
                          onPressed: isBusy
                              ? null
                              : () => _showCreateRepoDialog(domainProvider),
                          icon: const Icon(Icons.add_circle_outline, size: 16),
                          label: const Text('Создать'),
                          style: TextButton.styleFrom(
                            visualDensity: VisualDensity.compact,
                          ),
                        ),
                        TextButton.icon(
                          onPressed: isBusy
                              ? null
                              : () => setState(() => _manualUrl = true),
                          icon: const Icon(Icons.edit, size: 16),
                          label: const Text('URL вручную'),
                          style: TextButton.styleFrom(
                            visualDensity: VisualDensity.compact,
                          ),
                        ),
                      ],
                    ),
                  ),
                  IconButton(
                    icon: const Icon(Icons.refresh, size: 18),
                    tooltip: 'Обновить список',
                    onPressed: isBusy
                        ? null
                        : () =>
                            ref.invalidate(gitRepositoriesProvider(reposArgs)),
                  ),
                ],
              ),
            ],
          );
        },
        loading: () => const Padding(
          padding: EdgeInsets.symmetric(vertical: 16),
          child: Row(
            children: [
              SizedBox(
                width: 20,
                height: 20,
                child: CircularProgressIndicator(strokeWidth: 2),
              ),
              SizedBox(width: 12),
              Expanded(child: Text('Загрузка списка репозиториев...')),
            ],
          ),
        ),
        error: (err, _) => Container(
          padding: const EdgeInsets.all(12),
          decoration: BoxDecoration(
            color: theme.colorScheme.errorContainer,
            borderRadius: BorderRadius.circular(8),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Icon(Icons.error_outline,
                      color: theme.colorScheme.error, size: 20),
                  const SizedBox(width: 8),
                  const Expanded(
                    child: Text(
                      'Не удалось загрузить репозитории',
                      style: TextStyle(fontWeight: FontWeight.bold),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 8),
              Row(
                children: [
                  TextButton(
                    onPressed: () =>
                        ref.invalidate(gitRepositoriesProvider(reposArgs)),
                    child: const Text('Повторить'),
                  ),
                  const SizedBox(width: 8),
                  TextButton(
                    onPressed: () => setState(() => _manualUrl = true),
                    child: const Text('Ввести URL вручную'),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    ];
  }

  Future<void> _showCreateRepoDialog(GitIntegrationProvider provider) async {
    final nameCtrl = TextEditingController();
    final descCtrl = TextEditingController();
    var isPrivate = true;
    var isCreating = false;
    final dialogFormKey = GlobalKey<FormState>();

    await showDialog<void>(
      context: context,
      barrierDismissible: false,
      builder: (dialogCtx) {
        return StatefulBuilder(
          builder: (dialogCtx, setDialogState) {
            final theme = Theme.of(dialogCtx);
            return AlertDialog(
              title: Text(
                provider == GitIntegrationProvider.github
                    ? 'Создать репозиторий на GitHub'
                    : 'Создать репозиторий на GitLab',
              ),
              content: Form(
                key: dialogFormKey,
                child: SingleChildScrollView(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      TextFormField(
                        controller: nameCtrl,
                        decoration: const InputDecoration(
                          labelText: 'Имя репозитория',
                          hintText: 'my-awesome-repo',
                        ),
                        enabled: !isCreating,
                        validator: (v) {
                          final val = v?.trim() ?? '';
                          if (val.isEmpty) {
                            return 'Имя репозитория обязательно';
                          }
                          if (RegExp(r'[^a-zA-Z0-9._-]').hasMatch(val)) {
                            return 'Разрешены латиница, цифры, точки, дефисы и подчёркивания';
                          }
                          return null;
                        },
                      ),
                      const SizedBox(height: 16),
                      TextFormField(
                        controller: descCtrl,
                        decoration: const InputDecoration(
                          labelText: 'Описание (опционально)',
                        ),
                        enabled: !isCreating,
                        maxLines: 2,
                      ),
                      const SizedBox(height: 16),
                      SwitchListTile(
                        title: const Text('Приватный репозиторий'),
                        value: isPrivate,
                        onChanged: isCreating
                            ? null
                            : (val) => setDialogState(() => isPrivate = val),
                      ),
                    ],
                  ),
                ),
              ),
              actions: [
                TextButton(
                  onPressed:
                      isCreating ? null : () => Navigator.of(dialogCtx).pop(),
                  child: const Text('Отмена'),
                ),
                FilledButton(
                  onPressed: isCreating
                      ? null
                      : () async {
                          if (!dialogFormKey.currentState!.validate()) {
                            return;
                          }
                          setDialogState(() => isCreating = true);
                          try {
                            final newRepo = await ref
                                .read(gitIntegrationsRepositoryProvider)
                                .createRepository(
                                  provider,
                                  nameCtrl.text.trim(),
                                  accountId: _accountId,
                                  private: isPrivate,
                                  description: descCtrl.text.trim().isNotEmpty
                                      ? descCtrl.text.trim()
                                      : null,
                                );
                            ref.invalidate(gitRepositoriesProvider(
                                (provider: provider, accountId: _accountId)));
                            if (dialogCtx.mounted) {
                              Navigator.of(dialogCtx).pop();
                            }
                            if (mounted) {
                              setState(() {
                                _selectedRepo = newRepo;
                                _urlCtrl.text = newRepo.cloneUrl;
                                _manualUrl = false;
                              });
                              _notify();
                              ScaffoldMessenger.of(context).showSnackBar(
                                SnackBar(
                                  content: Text(
                                    'Репозиторий "${newRepo.name}" создан',
                                  ),
                                ),
                              );
                            }
                          } catch (err) {
                            setDialogState(() => isCreating = false);
                            if (dialogCtx.mounted) {
                              ScaffoldMessenger.of(dialogCtx).showSnackBar(
                                SnackBar(
                                  content: Text('Ошибка при создании: $err'),
                                  backgroundColor: theme.colorScheme.error,
                                ),
                              );
                            }
                          }
                        },
                  child: isCreating
                      ? const SizedBox(
                          width: 20,
                          height: 20,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : const Text('Создать'),
                ),
              ],
            );
          },
        );
      },
    );
    nameCtrl.dispose();
    descCtrl.dispose();
  }
}
