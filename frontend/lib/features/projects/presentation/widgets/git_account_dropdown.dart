import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/features/integrations/git/data/git_integrations_providers.dart';
import 'package:frontend/features/integrations/git/domain/git_integration_model.dart';
import 'package:frontend/l10n/app_localizations.dart';

/// Дропдаун выбора OAuth-аккаунта провайдера (мульти-аккаунт).
///
/// Показывает подключённые аккаунты провайдера [providerJsonValue] ('github'/'gitlab')
/// + пункт «по умолчанию» (null → бэк возьмёт первый аккаунт провайдера). Для провайдеров
/// без OAuth-аккаунтов (local/bitbucket) рендерит пустоту.
class GitAccountDropdown extends ConsumerWidget {
  const GitAccountDropdown({
    super.key,
    required this.providerJsonValue,
    required this.selectedId,
    required this.onChanged,
  });

  /// 'github' | 'gitlab' | 'bitbucket' | 'local' — git_provider проекта/репо.
  final String providerJsonValue;
  final String? selectedId;
  final ValueChanged<String?> onChanged;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final provider = GitIntegrationProvider.tryParse(providerJsonValue);
    if (provider == null) {
      // local / bitbucket — OAuth-аккаунтов нет.
      return const SizedBox.shrink();
    }
    final asyncAccounts = ref.watch(gitAccountsProvider);
    return asyncAccounts.when(
      loading: () => const SizedBox.shrink(),
      error: (_, _) => const SizedBox.shrink(),
      data: (accounts) {
        final forProvider = accounts
            .where((a) =>
                a.provider == provider &&
                a.status == GitProviderConnectionStatus.connected &&
                a.id != null)
            .toList();
        // selectedId, которого больше нет среди аккаунтов, нормализуем в null (default).
        final hasSelected =
            selectedId != null && forProvider.any((a) => a.id == selectedId);
        final value = hasSelected ? selectedId : null;
        return DropdownButtonFormField<String?>(
          // ignore: deprecated_member_use
          value: value,
          decoration: InputDecoration(
            labelText: l10n.gitAccountFieldLabel,
            helperText: forProvider.isEmpty
                ? l10n.gitAccountNoneHint
                : l10n.gitAccountHelper,
          ),
          items: [
            DropdownMenuItem<String?>(
              value: null,
              child: Text(l10n.gitAccountDefaultOption),
            ),
            for (final a in forProvider)
              DropdownMenuItem<String?>(
                value: a.id,
                child: Text(_label(a)),
              ),
          ],
          onChanged: onChanged,
        );
      },
    );
  }

  String _label(GitProviderConnection a) {
    final login = a.accountLogin ?? '';
    final host = a.host ?? '';
    if (host.isNotEmpty) {
      return login.isNotEmpty ? '$login @ $host' : host;
    }
    return login.isNotEmpty ? login : (a.id ?? '');
  }
}
