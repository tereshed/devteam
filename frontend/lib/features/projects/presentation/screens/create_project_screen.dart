import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/features/projects/domain/models.dart';
import 'package:frontend/features/projects/domain/requests.dart';
import 'package:frontend/features/projects/presentation/controllers/create_project_controller.dart';
import 'package:frontend/features/projects/presentation/utils/git_provider_display.dart';
import 'package:frontend/features/projects/presentation/utils/git_remote_url.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';

const int _kProjectNameMaxLength = 255;

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
                    Text(
                      detail,
                      style: theme.textTheme.bodySmall,
                    ),
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
                    maxLength: _kProjectNameMaxLength,
                    textInputAction: TextInputAction.next,
                    validator: (v) {
                      final t = v?.trim() ?? '';
                      if (t.isEmpty) {
                        return l10n.projectNameRequired;
                      }
                      if (t.length > _kProjectNameMaxLength) {
                        return l10n.projectNameMaxLength(_kProjectNameMaxLength);
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
                              if (!_isRemoteProvider(v)) {
                                _urlCtrl.clear();
                              }
                            });
                          },
                  ),
                  if (_isRemoteProvider(_gitProvider)) ...[
                    const SizedBox(height: 16),
                    TextFormField(
                      controller: _urlCtrl,
                      decoration: InputDecoration(
                        labelText: l10n.gitUrlFieldLabel,
                        hintText: l10n.gitUrlFieldHint,
                      ),
                      keyboardType: TextInputType.url,
                      textInputAction: TextInputAction.done,
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
}
