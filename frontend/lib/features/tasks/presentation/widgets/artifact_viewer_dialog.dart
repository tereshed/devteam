import 'dart:convert';

import 'package:flutter/foundation.dart' show compute;
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/tasks/data/orchestration_v2_providers.dart';
import 'package:frontend/features/tasks/domain/models/artifact_model.dart';
import 'package:frontend/shared/widgets/diff_viewer.dart';

/// Performance-критичные пороги для рендера full artifact (6.4).
///
/// На `code_diff` агенты выдают 5K-15K строк; на `merged_code` — целые SHA-блоки
/// и raw output'ы. Цель — не зависнуть на UI-thread'е при первом open'е диалога.
///
/// • [kArtifactJsonTruncationChars] — после этого порога JSON-вьюер показывает
///   только префикс + кнопку "Show full" (открывает полноэкранный sub-dialog).
/// • [kArtifactFailuresPageSize] / [kArtifactFailuresPaginationThreshold] —
///   `test_result.failures[]` > threshold пагинируется по pageSize, иначе
///   рендерится как обычно (overhead не оправдан).
const int kArtifactJsonTruncationChars = 50000;
const int kArtifactFailuresPaginationThreshold = 50;
const int kArtifactFailuresPageSize = 20;

/// Public entry-point: открывает диалог-просмотрщик артефакта по (taskId, artifactId).
///
/// Сам диалог watch'ит [artifactDetailProvider], так что повторный open того же
/// артефакта в рамках autoDispose-окна не сделает второй HTTP.
Future<void> showArtifactViewerDialog(
  BuildContext context, {
  required String taskId,
  required String artifactId,
}) {
  return showDialog<void>(
    context: context,
    builder: (ctx) => ArtifactViewerDialog(
      taskId: taskId,
      artifactId: artifactId,
    ),
  );
}

String _shortArtifactId(String id) =>
    id.length > 8 ? '${id.substring(0, 8)}…' : id;

class ArtifactViewerDialog extends ConsumerWidget {
  const ArtifactViewerDialog({
    super.key,
    required this.taskId,
    required this.artifactId,
  });

  final String taskId;
  final String artifactId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n =
        requireAppLocalizations(context, where: 'artifact_viewer_dialog');
    final async =
        ref.watch(artifactDetailProvider((taskId, artifactId)));

    return Dialog(
      insetPadding: const EdgeInsets.all(16),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 1100, maxHeight: 800),
        child: async.when(
          loading: () => const Padding(
            padding: EdgeInsets.all(24),
            child: Center(child: CircularProgressIndicator()),
          ),
          error: (err, _) => _DialogShell(
            title: l10n.artifactViewerTitle('?', _shortArtifactId(artifactId)),
            body: Padding(
              padding: const EdgeInsets.all(16),
              child: Text(
                l10n.artifactViewerLoadFailed('$err'),
                style: const TextStyle(color: Colors.red),
              ),
            ),
          ),
          data: (artifact) => _ArtifactViewerBody(artifact: artifact),
        ),
      ),
    );
  }
}

class _ArtifactViewerBody extends StatelessWidget {
  const _ArtifactViewerBody({required this.artifact});

  final Artifact artifact;

  @override
  Widget build(BuildContext context) {
    final l10n =
        requireAppLocalizations(context, where: 'artifact_viewer_dialog');
    final title = l10n.artifactViewerTitle(
        artifact.kind, _shortArtifactId(artifact.id));
    return _DialogShell(
      title: title,
      body: _bodyForKind(context, artifact),
    );
  }

  Widget _bodyForKind(BuildContext context, Artifact artifact) {
    final content = artifact.content;
    if (content == null || content.isEmpty) {
      final l10n =
          requireAppLocalizations(context, where: 'artifact_viewer_dialog');
      return Padding(
        padding: const EdgeInsets.all(16),
        child: Text(l10n.artifactViewerEmpty),
      );
    }
    switch (artifact.kind) {
      case 'code_diff':
        return _CodeDiffView(content: content);
      case 'review':
        return _ReviewView(content: content);
      case 'test_result':
        return _TestResultView(content: content);
      default:
        return _JsonView(content: content, kind: artifact.kind);
    }
  }
}

class _DialogShell extends StatelessWidget {
  const _DialogShell({required this.title, required this.body});

  final String title;
  final Widget body;

  @override
  Widget build(BuildContext context) {
    final l10n =
        requireAppLocalizations(context, where: 'artifact_viewer_dialog');
    return Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(20, 16, 8, 8),
          child: Row(
            children: [
              Expanded(
                child: Text(
                  title,
                  style: Theme.of(context).textTheme.titleMedium?.copyWith(
                        fontFamily: 'monospace',
                      ),
                ),
              ),
              IconButton(
                tooltip: l10n.artifactViewerClose,
                icon: const Icon(Icons.close),
                onPressed: () => Navigator.of(context).pop(),
              ),
            ],
          ),
        ),
        const Divider(height: 1),
        Flexible(child: body),
      ],
    );
  }
}

// ---------- code_diff ----------

class _CodeDiffView extends StatelessWidget {
  const _CodeDiffView({required this.content});

  final Map<String, dynamic> content;

  String _extractDiff() {
    final d = content['diff'];
    if (d is String) {
      return d;
    }
    // Не у всех генераторов поле `diff` — fallback на patch / unified.
    final p = content['patch'];
    if (p is String) {
      return p;
    }
    return '';
  }

  @override
  Widget build(BuildContext context) {
    final diff = _extractDiff();
    if (diff.isEmpty) {
      return _JsonView(content: content, kind: 'code_diff');
    }
    return Padding(
      padding: const EdgeInsets.all(12),
      child: DiffViewer(
        diff: diff,
        // На full-screen диалоге даём ему больше места.
        maxHeight: 680,
      ),
    );
  }
}

// ---------- review ----------

class _ReviewView extends StatelessWidget {
  const _ReviewView({required this.content});

  final Map<String, dynamic> content;

  List<String> _issues() {
    final raw = content['issues'];
    if (raw is! List) {
      return const [];
    }
    return raw
        .map((e) {
          if (e is String) {
            return e;
          }
          if (e is Map) {
            final msg = e['message'] ?? e['detail'] ?? e['summary'];
            if (msg is String && msg.isNotEmpty) {
              return msg;
            }
            return const JsonEncoder.withIndent('  ').convert(e);
          }
          return e.toString();
        })
        .whereType<String>()
        .toList(growable: false);
  }

  @override
  Widget build(BuildContext context) {
    final l10n =
        requireAppLocalizations(context, where: 'artifact_viewer_dialog');
    final decision = content['decision'] as String? ?? '—';
    final summary = content['summary'] as String? ?? '';
    final issues = _issues();
    final scheme = Theme.of(context).colorScheme;
    Color chipColor;
    switch (decision.toLowerCase()) {
      case 'approved':
      case 'approve':
        chipColor = Colors.green;
        break;
      case 'changes_requested':
      case 'rejected':
        chipColor = Colors.orange;
        break;
      default:
        chipColor = scheme.onSurfaceVariant;
    }
    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        Row(
          children: [
            Text(
              '${l10n.artifactViewerReviewDecision}: ',
              style: Theme.of(context).textTheme.titleSmall,
            ),
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
              decoration: BoxDecoration(
                color: chipColor.withValues(alpha: 0.15),
                borderRadius: BorderRadius.circular(12),
                border: Border.all(color: chipColor.withValues(alpha: 0.6)),
              ),
              child: Text(
                decision,
                style: TextStyle(
                  color: chipColor,
                  fontWeight: FontWeight.w600,
                  fontFamily: 'monospace',
                ),
              ),
            ),
          ],
        ),
        const SizedBox(height: 16),
        Text(l10n.artifactViewerReviewIssues,
            style: Theme.of(context).textTheme.titleSmall),
        const SizedBox(height: 8),
        if (issues.isEmpty)
          Text(l10n.artifactViewerReviewNoIssues,
              style: Theme.of(context).textTheme.bodySmall)
        else
          Table(
            border: TableBorder.all(
                color: scheme.outlineVariant, width: 0.5),
            columnWidths: const {
              0: IntrinsicColumnWidth(),
              1: FlexColumnWidth(),
            },
            children: [
              for (var i = 0; i < issues.length; i++)
                TableRow(children: [
                  Padding(
                    padding: const EdgeInsets.all(8),
                    child: Text('#${i + 1}',
                        style: const TextStyle(
                            fontFamily: 'monospace',
                            fontWeight: FontWeight.w600)),
                  ),
                  Padding(
                    padding: const EdgeInsets.all(8),
                    child: SelectableText(issues[i]),
                  ),
                ]),
            ],
          ),
        const SizedBox(height: 16),
        Text(l10n.artifactViewerReviewSummary,
            style: Theme.of(context).textTheme.titleSmall),
        const SizedBox(height: 8),
        SelectableText(summary.isEmpty ? '—' : summary),
      ],
    );
  }
}

// ---------- test_result ----------

class _TestResultView extends StatefulWidget {
  const _TestResultView({required this.content});

  final Map<String, dynamic> content;

  @override
  State<_TestResultView> createState() => _TestResultViewState();
}

class _TestResultViewState extends State<_TestResultView> {
  int _failuresShown = kArtifactFailuresPageSize;

  int _intField(String key) {
    final v = widget.content[key];
    if (v is num) {
      return v.toInt();
    }
    return 0;
  }

  List<Map<String, dynamic>> _failures() {
    final raw = widget.content['failures'];
    if (raw is! List) {
      return const [];
    }
    return raw
        .whereType<Map<String, dynamic>>()
        .toList(growable: false);
  }

  @override
  Widget build(BuildContext context) {
    final l10n =
        requireAppLocalizations(context, where: 'artifact_viewer_dialog');
    final passed = _intField('passed');
    final failed = _intField('failed');
    final skipped = _intField('skipped');
    final durationMs = _intField('duration_ms');
    final failures = _failures();
    final paginated =
        failures.length > kArtifactFailuresPaginationThreshold;
    final shown = paginated
        ? failures.take(_failuresShown).toList(growable: false)
        : failures;
    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        Wrap(
          spacing: 12,
          runSpacing: 8,
          children: [
            _StatChip(
                label: l10n.artifactViewerTestPassed,
                value: '$passed',
                color: Colors.green),
            _StatChip(
                label: l10n.artifactViewerTestFailed,
                value: '$failed',
                color: failed > 0 ? Colors.red : Colors.grey),
            _StatChip(
                label: l10n.artifactViewerTestSkipped,
                value: '$skipped',
                color: Colors.orange),
            _StatChip(
                label: l10n.artifactViewerTestDuration,
                value: l10n.artifactViewerTestDurationMs(durationMs),
                color: Theme.of(context).colorScheme.primary),
          ],
        ),
        const SizedBox(height: 16),
        if (failures.isEmpty)
          Text(l10n.artifactViewerTestNoFailures,
              style: Theme.of(context).textTheme.bodySmall)
        else ...[
          Text(
            l10n.artifactViewerTestFailuresHeader(failures.length),
            style: Theme.of(context).textTheme.titleSmall,
          ),
          const SizedBox(height: 8),
          for (final f in shown) _FailureTile(failure: f),
          if (paginated && _failuresShown < failures.length) ...[
            const SizedBox(height: 8),
            Align(
              alignment: Alignment.centerLeft,
              child: OutlinedButton(
                onPressed: () => setState(() {
                  _failuresShown = (_failuresShown + kArtifactFailuresPageSize)
                      .clamp(0, failures.length);
                }),
                child: Text(
                  l10n.artifactViewerShowNext(
                    (failures.length - _failuresShown)
                        .clamp(0, kArtifactFailuresPageSize),
                  ),
                ),
              ),
            ),
          ],
        ],
      ],
    );
  }
}

class _StatChip extends StatelessWidget {
  const _StatChip(
      {required this.label, required this.value, required this.color});

  final String label;
  final String value;
  final Color color;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: color.withValues(alpha: 0.5)),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(label,
              style: TextStyle(color: color, fontWeight: FontWeight.w500)),
          const SizedBox(width: 6),
          Text(value,
              style: TextStyle(
                  color: color,
                  fontWeight: FontWeight.w700,
                  fontFamily: 'monospace')),
        ],
      ),
    );
  }
}

class _FailureTile extends StatelessWidget {
  const _FailureTile({required this.failure});

  final Map<String, dynamic> failure;

  @override
  Widget build(BuildContext context) {
    final l10n =
        requireAppLocalizations(context, where: 'artifact_viewer_dialog');
    final name =
        failure['test_name'] as String? ?? l10n.artifactViewerTestUnnamed;
    final file = failure['file'] as String?;
    final line = (failure['line'] as num?)?.toInt() ?? 0;
    final message = failure['message'] as String? ?? '';
    final stack = failure['stack_trace'] as String?;
    return Card(
      margin: const EdgeInsets.symmetric(vertical: 4),
      child: ExpansionTile(
        title: Text(name,
            style: const TextStyle(
                fontFamily: 'monospace', fontWeight: FontWeight.w600)),
        subtitle: file != null
            ? Text(
                l10n.artifactViewerTestFailureFile(file, line),
                style: const TextStyle(fontFamily: 'monospace', fontSize: 12),
              )
            : null,
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 0, 16, 12),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                if (message.isNotEmpty) SelectableText(message),
                if (stack != null && stack.isNotEmpty) ...[
                  const SizedBox(height: 8),
                  Container(
                    padding: const EdgeInsets.all(8),
                    decoration: BoxDecoration(
                      color: Theme.of(context)
                          .colorScheme
                          .surfaceContainerHighest,
                      borderRadius: BorderRadius.circular(4),
                    ),
                    child: SelectableText(
                      stack,
                      style: const TextStyle(
                          fontFamily: 'monospace', fontSize: 12),
                    ),
                  ),
                ],
              ],
            ),
          ),
        ],
      ),
    );
  }
}

// ---------- generic JSON view (merged_code, plan, raw_output, …) ----------

class _JsonView extends StatelessWidget {
  const _JsonView({required this.content, required this.kind});

  final Map<String, dynamic> content;
  final String kind;

  @override
  Widget build(BuildContext context) {
    final pretty = const JsonEncoder.withIndent('  ').convert(content);
    return _LargeTextView(text: pretty, kind: kind);
  }
}

/// Универсальный вьюер для больших pretty-JSON блоков с truncation и
/// "Copy full" — спасает UI thread на `merged_code` / `raw_output_truncated`,
/// где content легко уходит в сотни KB.
class _LargeTextView extends StatelessWidget {
  const _LargeTextView({required this.text, required this.kind});

  final String text;
  final String kind;

  Future<void> _copyAll(BuildContext context) async {
    final l10n =
        requireAppLocalizations(context, where: 'artifact_viewer_dialog');
    final messenger = ScaffoldMessenger.maybeOf(context);
    try {
      await Clipboard.setData(ClipboardData(text: text));
      messenger?.showSnackBar(
        SnackBar(content: Text(l10n.artifactViewerCopiedSnack(text.length))),
      );
    } catch (_) {
      messenger?.showSnackBar(
        SnackBar(content: Text(l10n.artifactViewerCopyFailedSnack)),
      );
    }
  }

  Future<void> _showFullScreen(BuildContext context) async {
    await showDialog<void>(
      context: context,
      builder: (ctx) => Dialog.fullscreen(
        child: _FullScreenText(text: text, kind: kind),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final l10n =
        requireAppLocalizations(context, where: 'artifact_viewer_dialog');
    final isLarge = text.length > kArtifactJsonTruncationChars;
    // Truncation: рендерим до порога, остальное — за кнопкой "Show full".
    // Не пытаемся показать всё inline на 200KB JSON'а — даже с ListView.builder
    // это десятки тысяч строк и заметный prepare cost.
    final display =
        isLarge ? text.substring(0, kArtifactJsonTruncationChars) : text;
    final lines = const LineSplitter().convert(display);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 8, 16, 4),
          child: Row(
            children: [
              if (isLarge)
                Expanded(
                  child: Text(
                    l10n.artifactViewerTruncatedNotice(
                      (kArtifactJsonTruncationChars / 1024).round(),
                      (text.length / 1024).round(),
                    ),
                    style: Theme.of(context).textTheme.bodySmall?.copyWith(
                          color: Theme.of(context).colorScheme.onSurfaceVariant,
                        ),
                  ),
                )
              else
                const Spacer(),
              if (isLarge) ...[
                const SizedBox(width: 8),
                OutlinedButton.icon(
                  icon: const Icon(Icons.unfold_more, size: 16),
                  label: Text(
                    l10n.artifactViewerShowFull((text.length / 1024).round()),
                  ),
                  onPressed: () => _showFullScreen(context),
                ),
              ],
              const SizedBox(width: 8),
              TextButton.icon(
                icon: const Icon(Icons.copy, size: 16),
                label: Text(kind.isEmpty
                    ? l10n.artifactViewerCopyFull
                    : l10n.artifactViewerCopyFullForKind(kind)),
                onPressed: () => _copyAll(context),
              ),
            ],
          ),
        ),
        const Divider(height: 1),
        Expanded(
          child: SelectionArea(
            child: ListView.builder(
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
              itemCount: lines.length,
              itemExtent: 18,
              itemBuilder: (context, i) {
                return Text(
                  lines[i],
                  maxLines: 1,
                  softWrap: false,
                  overflow: TextOverflow.clip,
                  style: const TextStyle(
                      fontFamily: 'monospace', fontSize: 12, height: 1.4),
                );
              },
            ),
          ),
        ),
      ],
    );
  }
}

/// Top-level (не closure) — обязательное условие для [compute]: Dart-isolate
/// принимает только pure-function entry-point'ы. Возвращает уже nested-список
/// — конструируем за один pass, чтобы избежать второй копии в build().
List<String> _splitLinesIsolate(String text) =>
    const LineSplitter().convert(text);

/// Порог, после которого split идёт в фоновый isolate.
///
/// На < 50K text синхронный split занимает единицы ms — overhead на spawn
/// isolate'а (≈10-30 ms на mobile) не оправдан. Выше — UI thread может
/// заметно подзаикнуться, особенно на 3-5 MB merged_code от LLM.
const int _kFullScreenIsolateThreshold = 50 * 1024;

class _FullScreenText extends StatefulWidget {
  const _FullScreenText({required this.text, required this.kind});

  final String text;
  final String kind;

  @override
  State<_FullScreenText> createState() => _FullScreenTextState();
}

class _FullScreenTextState extends State<_FullScreenText> {
  late Future<List<String>> _linesFuture;

  @override
  void initState() {
    super.initState();
    // Маленький текст — split inline (см. _kFullScreenIsolateThreshold).
    // Большой — в isolate через compute(): не блокируем UI на open-анимации.
    if (widget.text.length < _kFullScreenIsolateThreshold) {
      _linesFuture =
          Future<List<String>>.value(_splitLinesIsolate(widget.text));
    } else {
      _linesFuture = compute(_splitLinesIsolate, widget.text);
    }
  }

  Future<void> _copy(BuildContext context) async {
    final l10n =
        requireAppLocalizations(context, where: 'artifact_viewer_dialog');
    final messenger = ScaffoldMessenger.maybeOf(context);
    try {
      await Clipboard.setData(ClipboardData(text: widget.text));
      messenger?.showSnackBar(
        SnackBar(
            content:
                Text(l10n.artifactViewerCopiedSnack(widget.text.length))),
      );
    } catch (_) {
      messenger?.showSnackBar(
        SnackBar(content: Text(l10n.artifactViewerCopyFailedSnack)),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n =
        requireAppLocalizations(context, where: 'artifact_viewer_dialog');
    return Scaffold(
      appBar: AppBar(
        title: Text(
          l10n.artifactViewerFullTitle(widget.kind),
          style: const TextStyle(fontFamily: 'monospace'),
        ),
        actions: [
          IconButton(
            tooltip: l10n.artifactViewerCopyFull,
            icon: const Icon(Icons.copy),
            onPressed: () => _copy(context),
          ),
        ],
      ),
      body: FutureBuilder<List<String>>(
        future: _linesFuture,
        builder: (context, snap) {
          if (!snap.hasData) {
            return const Center(child: CircularProgressIndicator());
          }
          final lines = snap.data!;
          return SelectionArea(
            child: ListView.builder(
              padding: const EdgeInsets.all(12),
              itemCount: lines.length,
              itemExtent: 18,
              itemBuilder: (context, i) {
                return Text(
                  lines[i],
                  maxLines: 1,
                  softWrap: false,
                  overflow: TextOverflow.clip,
                  style: const TextStyle(
                      fontFamily: 'monospace', fontSize: 12, height: 1.4),
                );
              },
            ),
          );
        },
      ),
    );
  }
}
