import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/utils/responsive.dart';
import 'package:frontend/core/widgets/adaptive_layout.dart';
import 'package:frontend/features/auth/data/api_key_providers.dart';
import 'package:frontend/features/auth/domain/models.dart';
import 'package:frontend/features/auth/presentation/controllers/api_key_controller.dart';
import 'package:frontend/features/auth/presentation/widgets/logout_button.dart';
import 'package:frontend/l10n/app_localizations.dart';

/// ApiKeysScreen — экран управления API-ключами пользователя
class ApiKeysScreen extends ConsumerWidget {
  const ApiKeysScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final keysState = ref.watch(apiKeyControllerProvider);

    return Scaffold(
      appBar: AppBar(
        title: Text(l10n.apiKeysTitle),
        actions: const [LogoutButton()],
      ),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: () => _showCreateDialog(context, ref),
        icon: const Icon(Icons.add),
        label: Text(l10n.apiKeyCreate),
      ),
      body: SafeArea(
        child: SingleChildScrollView(
          child: Container(
            alignment: Alignment.topCenter,
            child: AdaptiveContainer(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  SizedBox(height: Spacing.medium(context)),
                  // Описание
                  Card(
                    child: Padding(
                      padding: Spacing.cardPadding(context),
                      child: Row(
                        children: [
                          Icon(
                            Icons.info_outline,
                            color: Theme.of(context).colorScheme.primary,
                          ),
                          SizedBox(width: Spacing.small(context)),
                          Expanded(
                            child: Text(
                              l10n.apiKeyDescription,
                              style: Theme.of(context).textTheme.bodyMedium,
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
                  SizedBox(height: Spacing.medium(context)),
                  // MCP Configuration
                  _MCPConfigCard(ref: ref),
                  SizedBox(height: Spacing.medium(context)),
                  // Список ключей
                  keysState.when(
                    data: (keys) {
                      if (keys.isEmpty) {
                        return _EmptyState(l10n: l10n);
                      }
                      return Column(
                        children: keys
                            .map(
                              (key) => _ApiKeyCard(
                                apiKey: key,
                                onRevoke: () => _revokeKey(context, ref, key),
                                onDelete: () => _deleteKey(context, ref, key),
                              ),
                            )
                            .toList(),
                      );
                    },
                    loading: () =>
                        const Center(child: CircularProgressIndicator()),
                    error: (error, _) => Center(
                      child: Column(
                        children: [
                          Text(l10n.dataLoadError),
                          SizedBox(height: Spacing.small(context)),
                          ElevatedButton(
                            onPressed: () =>
                                ref.invalidate(apiKeyControllerProvider),
                            child: Text(l10n.retry),
                          ),
                        ],
                      ),
                    ),
                  ),
                  SizedBox(height: Spacing.xLarge(context)),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }

  void _showCreateDialog(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final nameController = TextEditingController();
    String? selectedExpiry;

    showDialog(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setState) => AlertDialog(
          title: Text(l10n.apiKeyCreate),
          content: SizedBox(
            width: 400,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                TextField(
                  controller: nameController,
                  decoration: InputDecoration(
                    labelText: l10n.apiKeyName,
                    hintText: l10n.apiKeyNameHint,
                    border: const OutlineInputBorder(),
                  ),
                  autofocus: true,
                ),
                const SizedBox(height: 16),
                DropdownButtonFormField<String>(
                  initialValue: selectedExpiry,
                  decoration: InputDecoration(
                    labelText: l10n.apiKeyExpiry,
                    border: const OutlineInputBorder(),
                  ),
                  items: [
                    DropdownMenuItem(
                      value: null,
                      child: Text(l10n.apiKeyNoExpiry),
                    ),
                    DropdownMenuItem(
                      value: '30',
                      child: Text(l10n.apiKeyExpiry30Days),
                    ),
                    DropdownMenuItem(
                      value: '90',
                      child: Text(l10n.apiKeyExpiry90Days),
                    ),
                    DropdownMenuItem(
                      value: '365',
                      child: Text(l10n.apiKeyExpiry1Year),
                    ),
                  ],
                  onChanged: (value) {
                    setState(() => selectedExpiry = value);
                  },
                ),
              ],
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(dialogContext).pop(),
              child: Text(l10n.cancel),
            ),
            FilledButton(
              onPressed: () async {
                final name = nameController.text.trim();
                if (name.isEmpty) {
                  return;
                }

                Navigator.of(dialogContext).pop();

                int? expiresIn;
                if (selectedExpiry != null) {
                  expiresIn = int.parse(selectedExpiry!) * 24 * 60 * 60;
                }

                try {
                  final created = await ref
                      .read(apiKeyControllerProvider.notifier)
                      .createKey(name: name, expiresInSeconds: expiresIn);

                  if (context.mounted) {
                    _showRawKeyDialog(context, created.rawKey, name);
                  }
                } catch (e) {
                  if (context.mounted) {
                    ScaffoldMessenger.of(context).showSnackBar(
                      SnackBar(
                        content: Text('${l10n.errorUnknown} $e'),
                        backgroundColor: Theme.of(context).colorScheme.error,
                      ),
                    );
                  }
                }
              },
              child: Text(l10n.create),
            ),
          ],
        ),
      ),
    );
  }

  void _showRawKeyDialog(BuildContext context, String rawKey, String name) {
    final l10n = AppLocalizations.of(context)!;

    showDialog(
      context: context,
      barrierDismissible: false,
      builder: (dialogContext) => AlertDialog(
        title: Row(
          children: [
            Icon(Icons.warning_amber, color: Theme.of(context).colorScheme.error),
            const SizedBox(width: 8),
            Expanded(child: Text(l10n.apiKeyCreated)),
          ],
        ),
        content: SizedBox(
          width: 500,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(
                l10n.apiKeyCreatedWarning,
                style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                      color: Theme.of(context).colorScheme.error,
                      fontWeight: FontWeight.bold,
                    ),
              ),
              const SizedBox(height: 16),
              Container(
                padding: const EdgeInsets.all(12),
                decoration: BoxDecoration(
                  color: Theme.of(context)
                      .colorScheme
                      .surfaceContainerHighest,
                  borderRadius: BorderRadius.circular(8),
                  border: Border.all(
                    color: Theme.of(context).colorScheme.outline,
                  ),
                ),
                child: SelectableText(
                  rawKey,
                  style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                        fontFamily: 'monospace',
                        fontWeight: FontWeight.w600,
                      ),
                ),
              ),
              const SizedBox(height: 12),
              OutlinedButton.icon(
                onPressed: () {
                  Clipboard.setData(ClipboardData(text: rawKey));
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(content: Text(l10n.apiKeyCopied)),
                  );
                },
                icon: const Icon(Icons.copy),
                label: Text(l10n.apiKeyCopy),
              ),
            ],
          ),
        ),
        actions: [
          FilledButton(
            onPressed: () => Navigator.of(dialogContext).pop(),
            child: Text(l10n.apiKeyUnderstood),
          ),
        ],
      ),
    );
  }

  Future<void> _revokeKey(
    BuildContext context,
    WidgetRef ref,
    ApiKeyModel key,
  ) async {
    final l10n = AppLocalizations.of(context)!;
    final confirm = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(l10n.apiKeyRevokeTitle),
        content: Text(l10n.apiKeyRevokeConfirm),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: Text(l10n.cancel),
          ),
          FilledButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            style: FilledButton.styleFrom(
              backgroundColor: Theme.of(context).colorScheme.error,
            ),
            child: Text(l10n.apiKeyRevoke),
          ),
        ],
      ),
    );

    if (confirm == true) {
      try {
        await ref.read(apiKeyControllerProvider.notifier).revokeKey(key.id);
      } catch (e) {
        if (context.mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text('${l10n.errorUnknown} $e')),
          );
        }
      }
    }
  }

  Future<void> _deleteKey(
    BuildContext context,
    WidgetRef ref,
    ApiKeyModel key,
  ) async {
    final l10n = AppLocalizations.of(context)!;
    final confirm = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(l10n.apiKeyDeleteTitle),
        content: Text(l10n.apiKeyDeleteConfirm),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: Text(l10n.cancel),
          ),
          FilledButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            style: FilledButton.styleFrom(
              backgroundColor: Theme.of(context).colorScheme.error,
            ),
            child: Text(l10n.delete),
          ),
        ],
      ),
    );

    if (confirm == true) {
      try {
        await ref.read(apiKeyControllerProvider.notifier).deleteKey(key.id);
      } catch (e) {
        if (context.mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text('${l10n.errorUnknown} $e')),
          );
        }
      }
    }
  }
}

/// _MCPConfigCard — карточка с MCP-конфигурацией для копирования
class _MCPConfigCard extends ConsumerStatefulWidget {
  final WidgetRef ref;

  const _MCPConfigCard({required this.ref});

  @override
  ConsumerState<_MCPConfigCard> createState() => _MCPConfigCardState();
}

class _MCPConfigCardState extends ConsumerState<_MCPConfigCard> {
  MCPConfigModel? _config;
  bool _isLoading = false;
  String? _error;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;

    return Card(
      child: ExpansionTile(
        leading: Icon(
          Icons.settings_input_component,
          color: Theme.of(context).colorScheme.primary,
        ),
        title: Text(
          l10n.mcpConfigTitle,
          style: Theme.of(context).textTheme.titleMedium,
        ),
        subtitle: Text(
          l10n.mcpConfigDescription,
          style: Theme.of(context).textTheme.bodySmall,
        ),
        onExpansionChanged: (expanded) {
          if (expanded && _config == null && !_isLoading && _error == null) {
            _loadConfig();
          }
        },
        children: [
          Padding(
            padding: Spacing.cardPadding(context),
            child: _buildContent(context, l10n),
          ),
        ],
      ),
    );
  }

  Widget _buildContent(BuildContext context, AppLocalizations l10n) {
    if (_isLoading) {
      return const Center(child: CircularProgressIndicator());
    }

    if (_error != null) {
      return Column(
        children: [
          Text(
            _error!,
            style: TextStyle(color: Theme.of(context).colorScheme.error),
          ),
          SizedBox(height: Spacing.small(context)),
          ElevatedButton(
            onPressed: _loadConfig,
            child: Text(l10n.retry),
          ),
        ],
      );
    }

    if (_config == null) {
      return const SizedBox.shrink();
    }

    final configJson = const JsonEncoder.withIndent('  ').convert(_config!.config);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        // Инструкция
        Text(
          l10n.mcpConfigInstructions,
          style: Theme.of(context).textTheme.titleSmall,
        ),
        SizedBox(height: Spacing.small(context)),
        Text(l10n.mcpConfigStep1, style: Theme.of(context).textTheme.bodySmall),
        Text(l10n.mcpConfigStep2, style: Theme.of(context).textTheme.bodySmall),
        Text(l10n.mcpConfigStep3Cursor, style: Theme.of(context).textTheme.bodySmall),
        Text(l10n.mcpConfigStep3Claude, style: Theme.of(context).textTheme.bodySmall),
        Text(l10n.mcpConfigStep4, style: Theme.of(context).textTheme.bodySmall),
        SizedBox(height: Spacing.medium(context)),

        // JSON Config
        Container(
          padding: const EdgeInsets.all(12),
          decoration: BoxDecoration(
            color: Theme.of(context).colorScheme.surfaceContainerHighest,
            borderRadius: BorderRadius.circular(8),
            border: Border.all(
              color: Theme.of(context).colorScheme.outline.withValues(alpha: 0.2),
            ),
          ),
          child: SelectableText(
            configJson,
            style: TextStyle(
              fontFamily: 'monospace',
              fontSize: 12,
              color: Theme.of(context).colorScheme.onSurface,
            ),
          ),
        ),
        SizedBox(height: Spacing.medium(context)),

        // Кнопка копирования
        FilledButton.icon(
          onPressed: () => _copyToClipboard(configJson, l10n),
          icon: const Icon(Icons.content_copy),
          label: Text(l10n.mcpConfigCopy),
        ),
      ],
    );
  }

  Future<void> _loadConfig() async {
    setState(() {
      _isLoading = true;
      _error = null;
    });

    try {
      final repo = ref.read(apiKeyRepositoryProvider);
      final config = await repo.getMCPConfig();
      setState(() {
        _config = config;
        _isLoading = false;
      });
    } catch (e) {
      setState(() {
        _error = e.toString().contains('disabled')
            ? AppLocalizations.of(context)!.mcpConfigDisabled
            : AppLocalizations.of(context)!.mcpConfigLoadError;
        _isLoading = false;
      });
    }
  }

  Future<void> _copyToClipboard(String text, AppLocalizations l10n) async {
    await Clipboard.setData(ClipboardData(text: text));
    if (mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(l10n.mcpConfigCopied)),
      );
    }
  }
}

class _EmptyState extends StatelessWidget {
  final AppLocalizations l10n;

  const _EmptyState({required this.l10n});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(
        children: [
          const SizedBox(height: 48),
          Icon(
            Icons.vpn_key_off,
            size: 64,
            color: Theme.of(context).colorScheme.onSurfaceVariant,
          ),
          const SizedBox(height: 16),
          Text(
            l10n.apiKeyEmpty,
            style: Theme.of(context).textTheme.titleMedium?.copyWith(
                  color: Theme.of(context).colorScheme.onSurfaceVariant,
                ),
          ),
          const SizedBox(height: 8),
          Text(
            l10n.apiKeyEmptyHint,
            style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                  color: Theme.of(context).colorScheme.onSurfaceVariant,
                ),
            textAlign: TextAlign.center,
          ),
        ],
      ),
    );
  }
}

class _ApiKeyCard extends StatelessWidget {
  final ApiKeyModel apiKey;
  final VoidCallback onRevoke;
  final VoidCallback onDelete;

  const _ApiKeyCard({
    required this.apiKey,
    required this.onRevoke,
    required this.onDelete,
  });

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final isExpired =
        apiKey.expiresAt != null && apiKey.expiresAt!.isBefore(DateTime.now());

    return Card(
      margin: const EdgeInsets.only(bottom: 8),
      child: Padding(
        padding: Spacing.cardPadding(context),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(
                  Icons.vpn_key,
                  size: Spacing.iconSize(context),
                  color: isExpired
                      ? Theme.of(context).colorScheme.error
                      : Theme.of(context).colorScheme.primary,
                ),
                SizedBox(width: Spacing.small(context)),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        apiKey.name,
                        style:
                            Theme.of(context).textTheme.titleMedium?.copyWith(
                                  fontWeight: FontWeight.w600,
                                ),
                      ),
                      Text(
                        '${apiKey.keyPrefix}...',
                        style: Theme.of(context).textTheme.bodySmall?.copyWith(
                              fontFamily: 'monospace',
                              color: Theme.of(context)
                                  .colorScheme
                                  .onSurfaceVariant,
                            ),
                      ),
                    ],
                  ),
                ),
                if (isExpired)
                  Chip(
                    label: Text(
                      l10n.apiKeyExpired,
                      style: TextStyle(
                        color: Theme.of(context).colorScheme.onError,
                        fontSize: 12,
                      ),
                    ),
                    backgroundColor: Theme.of(context).colorScheme.error,
                    padding: EdgeInsets.zero,
                    visualDensity: VisualDensity.compact,
                  ),
                PopupMenuButton<String>(
                  onSelected: (value) {
                    if (value == 'revoke') {
                      onRevoke();
                    }
                    if (value == 'delete') {
                      onDelete();
                    }
                  },
                  itemBuilder: (context) => [
                    PopupMenuItem(
                      value: 'revoke',
                      child: Row(
                        children: [
                          const Icon(Icons.block, size: 20),
                          const SizedBox(width: 8),
                          Text(l10n.apiKeyRevoke),
                        ],
                      ),
                    ),
                    PopupMenuItem(
                      value: 'delete',
                      child: Row(
                        children: [
                          Icon(
                            Icons.delete,
                            size: 20,
                            color: Theme.of(context).colorScheme.error,
                          ),
                          const SizedBox(width: 8),
                          Text(
                            l10n.delete,
                            style: TextStyle(
                              color: Theme.of(context).colorScheme.error,
                            ),
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
              ],
            ),
            SizedBox(height: Spacing.small(context)),
            Wrap(
              spacing: 16,
              runSpacing: 4,
              children: [
                _MetaItem(
                  icon: Icons.calendar_today,
                  label: l10n.apiKeyCreatedAt,
                  value: _formatDate(apiKey.createdAt),
                ),
                if (apiKey.expiresAt != null)
                  _MetaItem(
                    icon: Icons.timer,
                    label: l10n.apiKeyExpiresAt,
                    value: _formatDate(apiKey.expiresAt!),
                  ),
                if (apiKey.lastUsedAt != null)
                  _MetaItem(
                    icon: Icons.access_time,
                    label: l10n.apiKeyLastUsed,
                    value: _formatDate(apiKey.lastUsedAt!),
                  ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  String _formatDate(DateTime date) {
    return '${date.day.toString().padLeft(2, '0')}.'
        '${date.month.toString().padLeft(2, '0')}.'
        '${date.year} '
        '${date.hour.toString().padLeft(2, '0')}:'
        '${date.minute.toString().padLeft(2, '0')}';
  }
}

class _MetaItem extends StatelessWidget {
  final IconData icon;
  final String label;
  final String value;

  const _MetaItem({
    required this.icon,
    required this.label,
    required this.value,
  });

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(
          icon,
          size: 14,
          color: Theme.of(context).colorScheme.onSurfaceVariant,
        ),
        const SizedBox(width: 4),
        Text(
          '$label: $value',
          style: Theme.of(context).textTheme.bodySmall?.copyWith(
                color: Theme.of(context).colorScheme.onSurfaceVariant,
              ),
        ),
      ],
    );
  }
}
