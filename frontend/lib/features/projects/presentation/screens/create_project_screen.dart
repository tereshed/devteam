import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/api/dio_providers.dart';
import 'package:frontend/features/integrations/git/data/git_integrations_providers.dart';
import 'package:frontend/features/integrations/git/data/git_repositories_provider.dart';
import 'package:frontend/features/integrations/git/domain/git_integration_model.dart';
import 'package:frontend/features/integrations/git/domain/git_repository_model.dart';
import 'package:frontend/features/integrations/git/presentation/widgets/connect_gitlab_host_dialog.dart';
import 'package:frontend/features/projects/domain/models.dart';
import 'package:frontend/features/projects/domain/requests.dart';
import 'package:frontend/features/projects/presentation/controllers/create_project_controller.dart';
import 'package:frontend/features/projects/presentation/utils/git_provider_display.dart';
import 'package:frontend/features/projects/presentation/utils/git_remote_url.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';
import 'package:url_launcher/url_launcher.dart';

/// Максимальная длина имени проекта в форме (совпадает с бэкендом; для тестов — §10.9).
const int kProjectNameFieldMaxLength = 255;

class CreateProjectScreen extends ConsumerStatefulWidget {
  const CreateProjectScreen({super.key});

  @override
  ConsumerState<CreateProjectScreen> createState() =>
      _CreateProjectScreenState();
}

class _CreateProjectScreenState extends ConsumerState<CreateProjectScreen> {
  final _formKey = GlobalKey<FormState>();
  final _nameCtrl = TextEditingController();
  final _descCtrl = TextEditingController();
  final _urlCtrl = TextEditingController();

  String _gitProvider = gitProviders.first;
  bool _attemptedSubmit = false;
  bool _manualUrl = false;
  GitRepositoryModel? _selectedRepo;

  GitIntegrationProvider? get _domainGitProvider {
    return GitIntegrationProvider.tryParse(_gitProvider);
  }

  @override
  void dispose() {
    _nameCtrl.dispose();
    _descCtrl.dispose();
    _urlCtrl.dispose();
    super.dispose();
  }

  bool _isRemoteProvider(String p) => p != kLocalGitProvider;

  Future<void> _onSubmit() async {
    setState(() => _attemptedSubmit = true);
    if (!_formKey.currentState!.validate()) {
      return;
    }

    final name = _nameCtrl.text.trim();
    if (name.isEmpty) {
      return;
    }

    final request = CreateProjectRequest(
      name: name,
      description: _descCtrl.text.trim(),
      gitProvider: _gitProvider,
      gitUrl: _isRemoteProvider(_gitProvider) ? _urlCtrl.text.trim() : '',
      vectorCollection: '',
    );

    final created = await ref
        .read(createProjectControllerProvider.notifier)
        .submit(request);

    if (!mounted || created == null) {
      return;
    }
    context.go('/projects/${created.id}');
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);
    final asyncCreate = ref.watch(createProjectControllerProvider);
    final isBusy = asyncCreate.isLoading;

    ref.listen(createProjectControllerProvider, (_, next) {
      if (!next.hasError) {
        return;
      }
      final err = next.error!;
      final title = createProjectErrorTitle(l10n, err);
      final detail = createProjectErrorDetail(err);
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: detail != null
              ? Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(title),
                    Text(detail, style: theme.textTheme.bodySmall),
                  ],
                )
              : Text(title),
        ),
      );
    });

    const maxWidth = 600.0;
    final padding = MediaQuery.paddingOf(context);
    final width = MediaQuery.sizeOf(context).width;
    final horizontalPadding = EdgeInsets.fromLTRB(
      16 + padding.left,
      16 + padding.top,
      16 + padding.right,
      24 + padding.bottom,
    );

    return Scaffold(
      appBar: AppBar(
        leading: IconButton(
          icon: const Icon(Icons.arrow_back),
          onPressed: () {
            if (context.canPop()) {
              context.pop();
            } else {
              context.go('/projects');
            }
          },
        ),
        title: Text(l10n.createProjectScreenTitle),
      ),
      body: Align(
        alignment: Alignment.topCenter,
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: maxWidth),
          child: SingleChildScrollView(
            padding: horizontalPadding,
            child: Form(
              key: _formKey,
              autovalidateMode: _attemptedSubmit
                  ? AutovalidateMode.always
                  : AutovalidateMode.disabled,
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  TextFormField(
                    controller: _nameCtrl,
                    decoration: InputDecoration(
                      labelText: l10n.projectNameFieldLabel,
                      hintText: l10n.projectNameFieldHint,
                    ),
                    maxLength: kProjectNameFieldMaxLength,
                    textInputAction: TextInputAction.next,
                    validator: (v) {
                      final t = v?.trim() ?? '';
                      if (t.isEmpty) {
                        return l10n.projectNameRequired;
                      }
                      if (t.length > kProjectNameFieldMaxLength) {
                        return l10n.projectNameMaxLength(
                          kProjectNameFieldMaxLength,
                        );
                      }
                      return null;
                    },
                  ),
                  const SizedBox(height: 16),
                  TextFormField(
                    controller: _descCtrl,
                    decoration: InputDecoration(
                      labelText: l10n.projectDescriptionFieldLabel,
                      hintText: l10n.projectDescriptionFieldHint,
                      alignLabelWithHint: true,
                    ),
                    minLines: 3,
                    maxLines: 6,
                  ),
                  const SizedBox(height: 16),
                  DropdownButtonFormField<String>(
                    initialValue: _gitProvider,
                    decoration: InputDecoration(
                      labelText: l10n.gitProviderFieldLabel,
                    ),
                    items: [
                      for (final p in gitProviders)
                        DropdownMenuItem(
                          value: p,
                          child: Text(gitProviderDisplayLabel(context, p)),
                        ),
                    ],
                    onChanged: isBusy
                        ? null
                        : (v) {
                            if (v == null) {
                              return;
                            }
                            setState(() {
                              _gitProvider = v;
                              _selectedRepo = null;
                              _manualUrl = false;
                              _urlCtrl.clear();
                            });
                          },
                  ),
                  if (_isRemoteProvider(_gitProvider)) ...[
                    const SizedBox(height: 16),
                    ..._buildRemoteGitUrlSection(context, theme, l10n, isBusy),
                  ],
                  SizedBox(height: width >= maxWidth ? 32 : 24),
                  FilledButton(
                    onPressed: isBusy ? null : _onSubmit,
                    child: isBusy
                        ? const SizedBox(
                            height: 22,
                            width: 22,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : Text(l10n.create),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }

  List<Widget> _buildRemoteGitUrlSection(
    BuildContext context,
    ThemeData theme,
    AppLocalizations l10n,
    bool isBusy,
  ) {
    final domainProvider = _domainGitProvider;
    if (domainProvider == null) {
      // Fallback for providers without api support (like bitbucket)
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
      ];
    }

    final integrationsState = ref.watch(gitIntegrationsControllerProvider);
    final connection = integrationsState.connections[domainProvider];
    final isConnected = connection?.status == GitProviderConnectionStatus.connected;
    final isConnecting = connection?.status == GitProviderConnectionStatus.pending;

    if (!isConnected) {
      return [
        Container(
          padding: const EdgeInsets.all(16),
          decoration: BoxDecoration(
            color: theme.colorScheme.surfaceContainerHighest.withOpacity(0.3),
            borderRadius: BorderRadius.circular(12),
            border: Border.all(
              color: theme.colorScheme.outlineVariant.withOpacity(0.5),
            ),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Icon(
                    domainProvider == GitIntegrationProvider.github
                        ? Icons.code
                        : Icons.alt_route,
                    color: theme.colorScheme.primary,
                  ),
                  const SizedBox(width: 8),
                  Text(
                    domainProvider == GitIntegrationProvider.github
                        ? 'Подключение GitHub'
                        : 'Подключение GitLab',
                    style: theme.textTheme.titleMedium?.copyWith(
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 8),
              Text(
                'Подключите ваш аккаунт, чтобы автоматически выбирать репозитории из списка или создавать новые прямо отсюда.',
                style: theme.textTheme.bodyMedium?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                ),
              ),
              const SizedBox(height: 16),
              if (isConnecting) ...[
                Row(
                  children: [
                    const SizedBox(
                      width: 16,
                      height: 16,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: Text(
                        'Ожидание подтверждения в браузере...',
                        style: theme.textTheme.bodySmall?.copyWith(
                          fontStyle: FontStyle.italic,
                        ),
                      ),
                    ),
                  ],
                ),
              ] else ...[
                Wrap(
                  spacing: 8,
                  runSpacing: 8,
                  children: [
                    if (domainProvider == GitIntegrationProvider.github)
                      FilledButton.icon(
                        onPressed: isBusy
                            ? null
                            : () => _onConnect(GitIntegrationProvider.github),
                        icon: const Icon(Icons.link, size: 18),
                        label: const Text('Подключить GitHub'),
                      ),
                    if (domainProvider == GitIntegrationProvider.gitlab) ...[
                      FilledButton.icon(
                        onPressed: isBusy
                            ? null
                            : () => _onConnect(GitIntegrationProvider.gitlab),
                        icon: const Icon(Icons.link, size: 18),
                        label: const Text('Подключить GitLab.com'),
                      ),
                      OutlinedButton.icon(
                        onPressed: isBusy ? null : _onConnectSelfHosted,
                        icon: const Icon(Icons.dns, size: 18),
                        label: const Text('Self-hosted GitLab'),
                      ),
                    ],
                  ],
                ),
              ],
            ],
          ),
        ),
        const SizedBox(height: 16),
        TextFormField(
          controller: _urlCtrl,
          decoration: InputDecoration(
            labelText: l10n.gitUrlFieldLabel,
            hintText: l10n.gitUrlFieldHint,
            helperText: 'Или подключите интеграцию выше для автоматического выбора',
          ),
          keyboardType: TextInputType.url,
          textInputAction: TextInputAction.done,
          enabled: !isBusy,
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
      ];
    }

    // Connected state
    if (_manualUrl) {
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
        const SizedBox(height: 8),
        Align(
          alignment: Alignment.centerLeft,
          child: TextButton.icon(
            onPressed: () {
              setState(() {
                _manualUrl = false;
              });
            },
            icon: const Icon(Icons.list, size: 16),
            label: const Text('Выбрать из списка моих репозиториев'),
          ),
        ),
      ];
    }

    // Show the repository selector using future provider
    final reposAsync = ref.watch(gitRepositoriesProvider(domainProvider));
    return [
      reposAsync.when(
        data: (repos) {
          final List<GitRepositoryModel> itemsList = List.from(repos);
          if (_selectedRepo != null &&
              !itemsList.any((r) => r.cloneUrl == _selectedRepo!.cloneUrl)) {
            itemsList.insert(0, _selectedRepo!);
          }

          return Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              DropdownButtonFormField<GitRepositoryModel>(
                value: _selectedRepo,
                decoration: const InputDecoration(
                  labelText: 'Репозиторий',
                  hintText: 'Выберите репозиторий',
                ),
                items: itemsList.map((repo) {
                  return DropdownMenuItem<GitRepositoryModel>(
                    value: repo,
                    child: Text(repo.fullName),
                  );
                }).toList(),
                onChanged: isBusy
                    ? null
                    : (repo) {
                        setState(() {
                          _selectedRepo = repo;
                          if (repo != null) {
                            _urlCtrl.text = repo.cloneUrl;
                          }
                        });
                      },
                validator: (v) {
                  if (v == null && !_manualUrl) {
                    return 'Пожалуйста, выберите репозиторий';
                  }
                  return null;
                },
              ),
              const SizedBox(height: 8),
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Wrap(
                    spacing: 8,
                    children: [
                      TextButton.icon(
                        onPressed: isBusy
                            ? null
                            : () => _showCreateRepoDialog(domainProvider),
                        icon: const Icon(Icons.add_circle_outline, size: 16),
                        label: const Text('Создать репозиторий'),
                        style: TextButton.styleFrom(
                          visualDensity: VisualDensity.compact,
                        ),
                      ),
                      TextButton.icon(
                        onPressed: isBusy
                            ? null
                            : () {
                                setState(() {
                                  _manualUrl = true;
                                });
                              },
                        icon: const Icon(Icons.edit, size: 16),
                        label: const Text('Ввести URL вручную'),
                        style: TextButton.styleFrom(
                          visualDensity: VisualDensity.compact,
                        ),
                      ),
                    ],
                  ),
                  IconButton(
                    icon: const Icon(Icons.refresh, size: 18),
                    tooltip: 'Обновить список',
                    onPressed: isBusy
                        ? null
                        : () {
                            ref.invalidate(gitRepositoriesProvider(domainProvider));
                          },
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
              Text('Загрузка списка репозиториев...'),
            ],
          ),
        ),
        error: (err, stack) => Container(
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
                  Icon(
                    Icons.error_outline,
                    color: theme.colorScheme.error,
                    size: 20,
                  ),
                  const SizedBox(width: 8),
                  const Expanded(
                    child: Text(
                      'Не удалось загрузить репозитории',
                      style: TextStyle(fontWeight: FontWeight.bold),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 4),
              Text(
                err.toString(),
                style: theme.textTheme.bodySmall?.copyWith(
                  color: theme.colorScheme.onErrorContainer,
                ),
              ),
              const SizedBox(height: 8),
              Row(
                children: [
                  TextButton(
                    onPressed: () {
                      ref.invalidate(gitRepositoriesProvider(domainProvider));
                    },
                    child: const Text('Повторить попытку'),
                  ),
                  const SizedBox(width: 8),
                  TextButton(
                    onPressed: () {
                      setState(() {
                        _manualUrl = true;
                      });
                    },
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

  String _redirectUri(WidgetRef ref, GitIntegrationProvider provider) {
    final base = ref.read(dioClientProvider).options.baseUrl;
    final trimmed = base.endsWith('/')
        ? base.substring(0, base.length - 1)
        : base;
    return '$trimmed/integrations/${provider.jsonValue}/auth/callback';
  }

  Future<void> _onConnect(GitIntegrationProvider provider) async {
    final controller = ref.read(gitIntegrationsControllerProvider.notifier);
    final String authorizeUrl;
    try {
      authorizeUrl = await controller.initConnection(
        provider,
        redirectUri: _redirectUri(ref, provider),
      );
    } catch (_) {
      return;
    }
    final uri = Uri.tryParse(authorizeUrl);
    if (uri == null) {
      controller.rollbackToDisconnected(provider);
      return;
    }
    final ok = await launchUrl(uri, mode: LaunchMode.externalApplication);
    if (ok) {
      controller.pollUntilSettled(provider);
      return;
    }
    controller.rollbackToDisconnected(provider);
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text('Не удалось открыть браузер. Откройте URL вручную: $uri'),
      ),
    );
  }

  Future<void> _onConnectSelfHosted() async {
    final redirectUri = _redirectUri(ref, GitIntegrationProvider.gitlab);
    final launched = await showAndLaunchConnectGitlabHost(
      context,
      ref,
      redirectUri: redirectUri,
    );
    if (launched) {
      ref.read(gitIntegrationsControllerProvider.notifier).pollUntilSettled(GitIntegrationProvider.gitlab);
    }
  }

  Future<void> _showCreateRepoDialog(GitIntegrationProvider provider) async {
    final nameCtrl = TextEditingController();
    final descCtrl = TextEditingController();
    bool isPrivate = true;
    bool isCreating = false;
    final dialogFormKey = GlobalKey<FormState>();

    await showDialog(
      context: context,
      barrierDismissible: false,
      builder: (context) {
        return StatefulBuilder(
          builder: (context, setDialogState) {
            final theme = Theme.of(context);
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
                          // Basic git name check
                          if (RegExp(r'[^a-zA-Z0-9._-]').hasMatch(val)) {
                            return 'Разрешены только латиница, цифры, точки, дефисы и подчеркивания';
                          }
                          return null;
                        },
                      ),
                      const SizedBox(height: 16),
                      TextFormField(
                        controller: descCtrl,
                        decoration: const InputDecoration(
                          labelText: 'Описание (опционально)',
                          hintText: 'Проект команды DevTeam',
                        ),
                        enabled: !isCreating,
                        maxLines: 2,
                      ),
                      const SizedBox(height: 16),
                      SwitchListTile(
                        title: const Text('Приватный репозиторий'),
                        subtitle: const Text(
                          'Доступен только вам и приглашенным участникам',
                        ),
                        value: isPrivate,
                        onChanged: isCreating
                            ? null
                            : (val) {
                                setDialogState(() {
                                  isPrivate = val;
                                });
                              },
                      ),
                    ],
                  ),
                ),
              ),
              actions: [
                TextButton(
                  onPressed: isCreating ? null : () => Navigator.of(context).pop(),
                  child: const Text('Отмена'),
                ),
                FilledButton(
                  onPressed: isCreating
                      ? null
                      : () async {
                          if (!dialogFormKey.currentState!.validate()) {
                            return;
                          }
                          setDialogState(() {
                            isCreating = true;
                          });

                          try {
                            final newRepo = await ref
                                .read(gitIntegrationsRepositoryProvider)
                                .createRepository(
                                  provider,
                                  nameCtrl.text.trim(),
                                  private: isPrivate,
                                  description: descCtrl.text.trim().isNotEmpty
                                      ? descCtrl.text.trim()
                                      : null,
                                );

                            // Invalidate the cache of repositories list
                            ref.invalidate(gitRepositoriesProvider(provider));

                            if (context.mounted) {
                              setState(() {
                                _selectedRepo = newRepo;
                                _urlCtrl.text = newRepo.cloneUrl;
                                _manualUrl = false;
                              });

                              ScaffoldMessenger.of(context).showSnackBar(
                                SnackBar(
                                  content: Text(
                                    'Репозиторий "${newRepo.name}" успешно создан!',
                                  ),
                                ),
                              );
                              Navigator.of(context).pop();
                            }
                          } catch (err) {
                            setDialogState(() {
                              isCreating = false;
                            });
                            if (context.mounted) {
                              ScaffoldMessenger.of(context).showSnackBar(
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
                          child: CircularProgressIndicator(
                            strokeWidth: 2,
                            color: Colors.white,
                          ),
                        )
                      : const Text('Создать'),
                ),
              ],
            );
          },
        );
      },
    );
  }
}
