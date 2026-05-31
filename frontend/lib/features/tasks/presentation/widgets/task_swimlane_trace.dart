import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/projects/domain/models/agent_model.dart';
import 'package:frontend/features/tasks/data/orchestration_v2_providers.dart';
import 'package:frontend/features/tasks/domain/models/artifact_model.dart';
import 'package:frontend/features/tasks/domain/models/router_decision_model.dart';
import 'package:frontend/features/tasks/presentation/widgets/task_execution_graph.dart';
import 'package:frontend/features/team/data/team_providers.dart';
import 'package:frontend/l10n/app_localizations.dart';

/// Swimlane-трейс выполнения задачи: дорожки агентов × ось времени.
///
/// В отличие от [TaskExecutionGraph] (граф решений «сверху-вниз»), трейс делает
/// видимыми ПАРАЛЛЕЛИЗМ (агенты, работавшие внахлёст) и ДЛИТЕЛЬНОСТЬ шагов.
/// Использует те же данные/провайдеры и тот же контракт [onAgentSelected], что
/// и граф, поэтому переиспользует общий инспектор экрана.
class TaskSwimlaneTrace extends ConsumerStatefulWidget {
  const TaskSwimlaneTrace({
    super.key,
    required this.projectId,
    required this.taskId,
    required this.taskState,
    required this.onAgentSelected,
    this.selectedAgentName,
    this.selectedAgentNodeId,
    this.assignedAgentName,
    this.assignedAgentRole,
  });

  final String projectId;
  final String taskId;
  final String taskState;
  final void Function(AgentNodeData node) onAgentSelected;
  final String? selectedAgentName;
  final String? selectedAgentNodeId;
  final String? assignedAgentName;
  final String? assignedAgentRole;

  @override
  ConsumerState<TaskSwimlaneTrace> createState() => _TaskSwimlaneTraceState();
}

class _TaskSwimlaneTraceState extends ConsumerState<TaskSwimlaneTrace>
    with SingleTickerProviderStateMixin {
  late final AnimationController _anim;

  @override
  void initState() {
    super.initState();
    _anim = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 1500),
    );
    final isWidgetTest =
        WidgetsBinding.instance.runtimeType.toString().contains('Test');
    if (!isWidgetTest) {
      _anim.repeat();
    }
  }

  @override
  void dispose() {
    _anim.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'task_swimlane_trace');
    final teamAsync = ref.watch(teamProvider(widget.projectId));
    final decisionsAsync = ref.watch(taskRouterDecisionsProvider(widget.taskId));
    final artifactsAsync = ref.watch(taskArtifactsProvider(widget.taskId));

    Widget loadError(Object err) => Center(
          child: Text(
            '${l10n.dataLoadError}: $err',
            style: TextStyle(color: Theme.of(context).colorScheme.error),
          ),
        );

    return teamAsync.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (err, _) => loadError(err),
      data: (team) => decisionsAsync.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (err, _) => loadError(err),
        data: (decisions) => artifactsAsync.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (err, _) => loadError(err),
          data: (artifacts) =>
              _buildTrace(context, l10n, decisions, artifacts, team.agents),
        ),
      ),
    );
  }

  Widget _buildTrace(
    BuildContext context,
    AppLocalizations l10n,
    List<RouterDecision> decisions,
    List<Artifact> artifacts,
    List<AgentModel> teamAgents,
  ) {
    if (decisions.isEmpty) {
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Text(
            l10n.taskTraceWaiting,
            textAlign: TextAlign.center,
            style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                  color: Theme.of(context).colorScheme.onSurfaceVariant,
                ),
          ),
        ),
      );
    }

    final nodes = buildAgentNodes(
      decisions: decisions,
      artifacts: artifacts,
      taskState: widget.taskState,
      assignedAgentName: widget.assignedAgentName,
      assignedAgentRole: widget.assignedAgentRole,
      teamAgents: teamAgents,
    );

    final model = _SwimlaneModel.build(
      nodes: nodes,
      decisions: decisions,
      artifacts: artifacts,
      taskActive: widget.taskState == 'active',
      l10n: l10n,
    );

    final scheme = Theme.of(context).colorScheme;
    final colors = _TraceColors.fromScheme(scheme);
    final width = MediaQuery.sizeOf(context).width;
    final gutter = width < 600 ? 96.0 : 132.0;

    return Column(
      children: [
        Expanded(
          child: LayoutBuilder(
            builder: (context, c) {
              final layout = _TraceLayout(
                size: Size(c.maxWidth, c.maxHeight),
                model: model,
                gutter: gutter,
              );
              final body = SizedBox(
                width: layout.size.width,
                height: layout.size.height,
                child: Stack(
                  children: [
                    Positioned.fill(
                      child: AnimatedBuilder(
                        animation: _anim,
                        builder: (_, _) => CustomPaint(
                          painter: _TracePainter(
                            layout: layout,
                            colors: colors,
                            t: _anim.value,
                            selectedNodeId: widget.selectedAgentNodeId,
                            selectedName: widget.selectedAgentName,
                          ),
                        ),
                      ),
                    ),
                    ..._buildOverlays(context, l10n, layout),
                  ],
                ),
              );
              // Вертикальный скролл, если дорожек больше, чем влезает по высоте.
              if (layout.contentH > c.maxHeight) {
                return SingleChildScrollView(child: body);
              }
              return body;
            },
          ),
        ),
        _TraceLegend(colors: colors, l10n: l10n),
      ],
    );
  }

  List<Widget> _buildOverlays(
    BuildContext context,
    AppLocalizations l10n,
    _TraceLayout layout,
  ) {
    final out = <Widget>[];
    // Спаны — кликабельны, ведут в общий инспектор (как агент-нода графа).
    for (final s in layout.model.spans) {
      final r = layout.spanRect(s);
      out.add(Positioned.fromRect(
        rect: r,
        child: Tooltip(
          message: s.tooltip,
          waitDuration: const Duration(milliseconds: 400),
          child: Material(
            type: MaterialType.transparency,
            child: InkWell(
              borderRadius: BorderRadius.circular(7),
              onTap: () => widget.onAgentSelected(s.node),
            ),
          ),
        ),
      ));
    }
    // Гейты роутера — ховер-тултип с reason (как reason на router-карточке графа).
    for (final g in layout.model.gates) {
      final c = Offset(layout.xOf(g.atSec), layout.laneCenter(0));
      out.add(Positioned(
        left: c.dx - 12,
        top: c.dy - 12,
        width: 24,
        height: 24,
        child: Tooltip(
          message: g.tooltip,
          waitDuration: const Duration(milliseconds: 300),
          child: const SizedBox.expand(),
        ),
      ));
    }
    return out;
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// COLORS — фон/линии/текст из темы (свет+тьма), статус-акценты как в графе.
// ─────────────────────────────────────────────────────────────────────────────
class _TraceColors {
  final Color surface;
  final Color laneStripe;
  final Color line;
  final Color lineSoft;
  final Color onSurface;
  final Color onSurfaceMuted;
  final Color router;
  const _TraceColors({
    required this.surface,
    required this.laneStripe,
    required this.line,
    required this.lineSoft,
    required this.onSurface,
    required this.onSurfaceMuted,
    required this.router,
  });

  factory _TraceColors.fromScheme(ColorScheme s) => _TraceColors(
        surface: s.surface,
        laneStripe: s.surfaceContainerHighest.withValues(alpha: 0.35),
        line: s.outlineVariant,
        lineSoft: s.outlineVariant.withValues(alpha: 0.5),
        onSurface: s.onSurface,
        onSurfaceMuted: s.onSurfaceVariant,
        router: s.primary,
      );

  static Color status(NodeStatus s) => switch (s) {
        NodeStatus.pending => Colors.grey,
        NodeStatus.running => Colors.blue,
        NodeStatus.success => Colors.green,
        NodeStatus.failed => Colors.red,
      };

  static String statusLabel(AppLocalizations l10n, NodeStatus s) => switch (s) {
        NodeStatus.pending => l10n.agentMatrixStatusPending,
        NodeStatus.running => l10n.agentMatrixStatusRunning,
        NodeStatus.success => l10n.agentMatrixStatusSuccess,
        NodeStatus.failed => l10n.agentMatrixStatusFailed,
      };
}

// ─────────────────────────────────────────────────────────────────────────────
// MODEL — lanes / spans / gates со временем в секундах от t0.
// ─────────────────────────────────────────────────────────────────────────────
class _Lane {
  final String key;
  final String label;
  final String sub;
  const _Lane(this.key, this.label, this.sub);
}

class _Span {
  final AgentNodeData node;
  final String laneKey;
  final double startSec;
  final double endSec;
  final bool running;
  final NodeStatus status;
  final String label; // короткая подпись на баре
  final String tooltip;
  const _Span({
    required this.node,
    required this.laneKey,
    required this.startSec,
    required this.endSec,
    required this.running,
    required this.status,
    required this.label,
    required this.tooltip,
  });
}

class _Gate {
  final int stepNo;
  final double atSec;
  final NodeStatus status;
  final String tooltip;
  const _Gate(this.stepNo, this.atSec, this.status, this.tooltip);
}

class _SwimlaneModel {
  final List<_Lane> lanes; // [0] всегда router
  final List<_Span> spans;
  final List<_Gate> gates;
  final List<(String fromNodeId, String toNodeId)> deps;
  final double maxSec;
  final List<double> stepTicksSec;

  const _SwimlaneModel({
    required this.lanes,
    required this.spans,
    required this.gates,
    required this.deps,
    required this.maxSec,
    required this.stepTicksSec,
  });

  static const routerLaneKey = '__router__';

  static _SwimlaneModel build({
    required List<AgentNodeData> nodes,
    required List<RouterDecision> decisions,
    required List<Artifact> artifacts,
    required bool taskActive,
    required AppLocalizations l10n,
  }) {
    final sorted = List<RouterDecision>.from(decisions)
      ..sort((a, b) {
        final byStep = a.stepNo.compareTo(b.stepNo);
        return byStep != 0 ? byStep : a.createdAt.compareTo(b.createdAt);
      });

    // t0 — самое раннее событие; tEnd — now (если активна) или последнее событие.
    var t0 = sorted.first.createdAt;
    var tEnd = sorted.last.createdAt;
    for (final a in artifacts) {
      if (a.createdAt.isBefore(t0)) {
        t0 = a.createdAt;
      }
      if (a.createdAt.isAfter(tEnd)) {
        tEnd = a.createdAt;
      }
    }
    final now = DateTime.now();
    if (taskActive && now.isAfter(tEnd)) {
      tEnd = now;
    }
    double sec(DateTime d) => d.difference(t0).inMilliseconds / 1000.0;
    final tEndSec = math.max(sec(tEnd), 1.0);

    // Окна шагов: [createdAt, следующий createdAt) либо до tEnd для последнего.
    final winStart = <int, double>{};
    final winEnd = <int, double>{};
    for (var i = 0; i < sorted.length; i++) {
      winStart[sorted[i].stepNo] = sec(sorted[i].createdAt);
      winEnd[sorted[i].stepNo] =
          i + 1 < sorted.length ? sec(sorted[i + 1].createdAt) : tEndSec;
    }

    // Дорожки: router сверху, далее агенты в порядке первого появления.
    final lanes = <_Lane>[_Lane(routerLaneKey, l10n.taskTraceRouterLane, '')];
    final firstStep = <String, int>{};
    final roleOf = <String, String>{};
    for (final n in nodes) {
      if (n.kind != NodeKind.agent) {
        continue;
      }
      firstStep.putIfAbsent(n.name, () => n.stepNo);
      roleOf.putIfAbsent(n.name, () => n.role);
    }
    final agentNames = firstStep.keys.toList()
      ..sort((a, b) {
        final byStep = firstStep[a]!.compareTo(firstStep[b]!);
        return byStep != 0 ? byStep : a.compareTo(b);
      });
    for (final name in agentNames) {
      lanes.add(_Lane(name, name, roleOf[name] ?? ''));
    }

    // Спаны агентов + карта artifactId → nodeId (для рёбер зависимостей).
    final spans = <_Span>[];
    final artifactToNode = <String, String>{};
    for (final n in nodes) {
      if (n.kind != NodeKind.agent) {
        continue;
      }
      final start = winStart[n.stepNo] ?? 0;
      final wEnd = winEnd[n.stepNo] ?? tEndSec;
      double end;
      final running = n.status == NodeStatus.running;
      if (running) {
        end = tEndSec;
      } else if (n.artifacts.isNotEmpty) {
        end = n.artifacts
            .map((a) => sec(a.createdAt))
            .reduce((a, b) => a > b ? a : b);
      } else {
        end = wEnd;
      }
      if (end < start) {
        end = start;
      }
      for (final a in n.artifacts) {
        artifactToNode[a.id] = n.id;
      }
      final kind = n.artifacts.isNotEmpty ? n.artifacts.last.kind : null;
      final label = kind ?? _TraceColors.statusLabel(l10n, n.status);
      final tip = StringBuffer(
          '${n.name} · step ${n.stepNo} · ${_TraceColors.statusLabel(l10n, n.status)}');
      if (n.artifacts.isNotEmpty) {
        tip.write('\n${n.artifacts.last.kind}: ${n.artifacts.last.summary}');
      }
      spans.add(_Span(
        node: n,
        laneKey: n.name,
        startSec: start,
        endSec: end,
        running: running,
        status: n.status,
        label: label,
        tooltip: tip.toString(),
      ));
    }

    // Гейты роутера.
    final gates = <_Gate>[];
    for (final n in nodes) {
      if (n.kind != NodeKind.router) {
        continue;
      }
      final at = winStart[n.stepNo] ?? 0;
      final tip = (n.reason != null && n.reason!.isNotEmpty)
          ? 'step ${n.stepNo}\n${n.reason}'
          : 'step ${n.stepNo}';
      gates.add(_Gate(n.stepNo, at, n.status, tip));
    }

    // Рёбра зависимостей артефактов (parent_id), только между разными спанами.
    final deps = <(String, String)>[];
    for (final a in artifacts) {
      final pid = a.parentId;
      if (pid == null) {
        continue;
      }
      final from = artifactToNode[pid];
      final to = artifactToNode[a.id];
      if (from != null && to != null && from != to) {
        deps.add((from, to));
      }
    }

    final stepTicks = sorted.map((d) => sec(d.createdAt)).toList();

    return _SwimlaneModel(
      lanes: lanes,
      spans: spans,
      gates: gates,
      deps: deps,
      maxSec: tEndSec * 1.05,
      stepTicksSec: stepTicks,
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// LAYOUT — геометрия (единый источник для отрисовки и оверлеев).
// ─────────────────────────────────────────────────────────────────────────────
class _TraceLayout {
  final Size size;
  final _SwimlaneModel model;
  final double gutter;
  static const double rulerH = 44;
  static const double laneH = 52;
  static const double barH = 28;
  static const double rightPad = 28;

  late final double pxPerSec;
  late final double topOffset;
  late final double contentH;

  _TraceLayout({required this.size, required this.model, required this.gutter}) {
    contentH = rulerH + model.lanes.length * laneH;
    pxPerSec = (size.width - gutter - rightPad) / model.maxSec;
    topOffset = math.max(8, (size.height - contentH) / 2);
  }

  double get bandTop => topOffset + rulerH;
  double get bandBottom => topOffset + contentH;
  int laneIndex(String key) => model.lanes.indexWhere((l) => l.key == key);
  double laneCenter(int i) => topOffset + rulerH + i * laneH + laneH / 2;
  double xOf(double sec) => gutter + sec * pxPerSec;

  Rect spanRect(_Span s) {
    final cy = laneCenter(laneIndex(s.laneKey));
    final x0 = xOf(s.startSec);
    final x1 = xOf(s.endSec);
    return Rect.fromLTRB(x0, cy - barH / 2, math.max(x1, x0 + 10), cy + barH / 2);
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// PAINTER
// ─────────────────────────────────────────────────────────────────────────────
class _TracePainter extends CustomPainter {
  _TracePainter({
    required this.layout,
    required this.colors,
    required this.t,
    required this.selectedNodeId,
    required this.selectedName,
  });
  final _TraceLayout layout;
  final _TraceColors colors;
  final double t;
  final String? selectedNodeId;
  final String? selectedName;

  _SwimlaneModel get m => layout.model;

  @override
  void paint(Canvas canvas, Size size) {
    _laneBackgrounds(canvas, size);
    _ruler(canvas, size);
    _gutter(canvas);
    _deps(canvas);
    _spans(canvas);
    _gates(canvas);
    _nowLine(canvas, size);
  }

  void _laneBackgrounds(Canvas canvas, Size size) {
    for (var i = 0; i < m.lanes.length; i++) {
      final cy = layout.laneCenter(i);
      if (i.isOdd) {
        canvas.drawRect(
          Rect.fromLTWH(layout.gutter, cy - _TraceLayout.laneH / 2, size.width,
              _TraceLayout.laneH),
          Paint()..color = colors.laneStripe,
        );
      }
      canvas.drawLine(
        Offset(0, cy + _TraceLayout.laneH / 2),
        Offset(size.width, cy + _TraceLayout.laneH / 2),
        Paint()..color = colors.lineSoft,
      );
    }
    canvas.drawLine(Offset(layout.gutter, layout.topOffset),
        Offset(layout.gutter, layout.bandBottom), Paint()..color = colors.line);
  }

  void _ruler(Canvas canvas, Size size) {
    for (final g in m.gates) {
      final x = layout.xOf(g.atSec);
      _dashedV(canvas, x, layout.bandTop, layout.bandBottom, colors.lineSoft);
      _text(canvas, 's${g.stepNo}', Offset(x + 5, layout.topOffset + 6),
          color: colors.onSurfaceMuted, size: 11, weight: FontWeight.w700);
    }
    // Тики времени каждую минуту.
    for (double s = 0; s <= m.maxSec; s += 60) {
      final x = layout.xOf(s);
      final mm = (s ~/ 60).toString();
      _text(canvas, '$mm:00', Offset(x + 5, layout.topOffset + 24),
          color: colors.onSurfaceMuted.withValues(alpha: 0.7), size: 9);
    }
    canvas.drawLine(Offset(layout.gutter, layout.bandTop),
        Offset(size.width, layout.bandTop), Paint()..color = colors.line);
  }

  void _gutter(Canvas canvas) {
    for (var i = 0; i < m.lanes.length; i++) {
      final lane = m.lanes[i];
      final cy = layout.laneCenter(i);
      final accent = i == 0 ? colors.router : colors.onSurfaceMuted;
      canvas.drawCircle(Offset(16, cy - 5), 4, Paint()..color = accent);
      _text(canvas, lane.label, Offset(28, cy - 13),
          color: colors.onSurface,
          size: 12.5,
          weight: FontWeight.w600,
          maxWidth: layout.gutter - 32);
      if (lane.sub.isNotEmpty) {
        _text(canvas, lane.sub, Offset(28, cy + 2),
            color: colors.onSurfaceMuted.withValues(alpha: 0.8),
            size: 10,
            maxWidth: layout.gutter - 32);
      }
    }
  }

  void _deps(Canvas canvas) {
    final byNode = <String, Rect>{};
    for (final s in m.spans) {
      byNode[s.node.id] = layout.spanRect(s);
    }
    for (final d in m.deps) {
      final a = byNode[d.$1];
      final b = byNode[d.$2];
      if (a == null || b == null) {
        continue;
      }
      final p1 = Offset(a.right, a.center.dy);
      final p2 = Offset(b.left, b.center.dy);
      final dx = (p2.dx - p1.dx).abs().clamp(24, 90) * 0.6;
      final path = Path()
        ..moveTo(p1.dx, p1.dy)
        ..cubicTo(p1.dx + dx, p1.dy, p2.dx - dx, p2.dy, p2.dx, p2.dy);
      canvas.drawPath(
          path,
          Paint()
            ..style = PaintingStyle.stroke
            ..strokeWidth = 1.4
            ..color = colors.onSurfaceMuted.withValues(alpha: 0.45));
      canvas.drawCircle(
          p2, 2.5, Paint()..color = colors.onSurfaceMuted.withValues(alpha: 0.6));
    }
  }

  bool _isSelected(_Span s) =>
      (selectedNodeId != null && selectedNodeId == s.node.id) ||
      (selectedNodeId == null &&
          selectedName != null &&
          selectedName == s.node.name);

  void _spans(Canvas canvas) {
    for (final s in m.spans) {
      final r = layout.spanRect(s);
      final c = _TraceColors.status(s.status);
      final rr = RRect.fromRectAndRadius(r, const Radius.circular(7));
      final selected = _isSelected(s);

      if (s.running) {
        canvas.drawRRect(
            RRect.fromRectAndRadius(r.inflate(3), const Radius.circular(9)),
            Paint()
              ..color = c.withValues(alpha: 0.12 + 0.10 * _pulse())
              ..maskFilter = const MaskFilter.blur(BlurStyle.normal, 5));
      }

      canvas.drawRRect(rr, Paint()..color = c.withValues(alpha: 0.20));

      if (s.running) {
        canvas.save();
        canvas.clipRRect(rr);
        final stripe = Paint()
          ..color = c.withValues(alpha: 0.22)
          ..strokeWidth = 6;
        final off = t * 18;
        for (var x = r.left - 20 + off; x < r.right + 20; x += 14) {
          canvas.drawLine(Offset(x, r.bottom), Offset(x + 18, r.top), stripe);
        }
        canvas.restore();
      }

      canvas.drawRRect(
          rr,
          Paint()
            ..style = PaintingStyle.stroke
            ..strokeWidth = selected ? 2.4 : 1.3
            ..color = selected ? colors.router : c.withValues(alpha: 0.75));

      if (selected) {
        canvas.drawRRect(
            RRect.fromRectAndRadius(r.inflate(3.5), const Radius.circular(9)),
            Paint()
              ..style = PaintingStyle.stroke
              ..strokeWidth = 1.2
              ..color = colors.router.withValues(alpha: 0.5));
      }

      final glyph = switch (s.status) {
        NodeStatus.success => '✓ ',
        NodeStatus.failed => '✕ ',
        _ => '',
      };
      _text(canvas, '$glyph${s.label}', Offset(r.left + 8, r.center.dy - 7),
          color: colors.onSurface,
          size: 11.5,
          weight: FontWeight.w600,
          maxWidth: r.width - 14);

      if (s.running) {
        canvas.drawCircle(Offset(r.right - 8, r.center.dy), 3.2,
            Paint()..color = c.withValues(alpha: 0.5 + 0.5 * _pulse()));
      }
    }
  }

  void _gates(Canvas canvas) {
    final ry = layout.laneCenter(0);
    for (final g in m.gates) {
      final x = layout.xOf(g.atSec);
      final col = g.status == NodeStatus.failed ? Colors.red : colors.router;
      final path = Path()
        ..moveTo(x, ry - 9)
        ..lineTo(x + 9, ry)
        ..lineTo(x, ry + 9)
        ..lineTo(x - 9, ry)
        ..close();
      canvas.drawPath(path, Paint()..color = col.withValues(alpha: 0.95));
      canvas.drawPath(
          path,
          Paint()
            ..style = PaintingStyle.stroke
            ..strokeWidth = 1.2
            ..color = colors.surface);
      _textCentered(canvas, '${g.stepNo}', Offset(x, ry),
          color: colors.surface, size: 10, weight: FontWeight.w800);
    }
  }

  void _nowLine(Canvas canvas, Size size) {
    if (!m.spans.any((s) => s.running)) {
      return;
    }
    final x = layout.xOf(m.maxSec / 1.05); // tEnd без 5% паддинга
    canvas.drawLine(
        Offset(x, layout.bandTop),
        Offset(x, layout.bandBottom),
        Paint()
          ..strokeWidth = 1.4
          ..color = Colors.blue.withValues(alpha: 0.4 + 0.35 * _pulse()));
    canvas.drawCircle(Offset(x, layout.bandTop), 3, Paint()..color = Colors.blue);
  }

  double _pulse() => 0.5 + 0.5 * math.sin(t * 2 * math.pi);

  void _text(Canvas canvas, String s, Offset o,
      {required Color color,
      required double size,
      FontWeight weight = FontWeight.w400,
      double? maxWidth}) {
    final tp = TextPainter(
      text: TextSpan(
        text: s,
        style: TextStyle(color: color, fontSize: size, fontWeight: weight),
      ),
      textDirection: TextDirection.ltr,
      maxLines: 1,
      ellipsis: '…',
    )..layout(
        maxWidth: maxWidth == null ? double.infinity : math.max(0.0, maxWidth));
    tp.paint(canvas, o);
  }

  void _textCentered(Canvas canvas, String s, Offset center,
      {required Color color,
      required double size,
      FontWeight weight = FontWeight.w400}) {
    final tp = TextPainter(
      text: TextSpan(
        text: s,
        style: TextStyle(color: color, fontSize: size, fontWeight: weight),
      ),
      textDirection: TextDirection.ltr,
    )..layout();
    tp.paint(canvas, center - Offset(tp.width / 2, tp.height / 2));
  }

  void _dashedV(Canvas canvas, double x, double y0, double y1, Color color) {
    final paint = Paint()
      ..color = color
      ..strokeWidth = 1;
    for (var y = y0; y < y1; y += 7) {
      canvas.drawLine(Offset(x, y), Offset(x, math.min(y + 4, y1)), paint);
    }
  }

  @override
  bool shouldRepaint(covariant _TracePainter old) =>
      old.t != t ||
      old.selectedNodeId != selectedNodeId ||
      old.selectedName != selectedName ||
      old.layout != layout;
}

// ─────────────────────────────────────────────────────────────────────────────
// LEGEND
// ─────────────────────────────────────────────────────────────────────────────
class _TraceLegend extends StatelessWidget {
  const _TraceLegend({required this.colors, required this.l10n});
  final _TraceColors colors;
  final AppLocalizations l10n;

  @override
  Widget build(BuildContext context) {
    Widget swatch(Color c) => Container(
          width: 16,
          height: 9,
          decoration: BoxDecoration(
            color: c.withValues(alpha: 0.25),
            borderRadius: BorderRadius.circular(3),
            border: Border.all(color: c.withValues(alpha: 0.8)),
          ),
        );
    Widget item(Widget glyph, String label) => Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            glyph,
            const SizedBox(width: 6),
            Text(label,
                style: TextStyle(color: colors.onSurfaceMuted, fontSize: 11)),
          ],
        );
    return Container(
      height: 38,
      decoration: BoxDecoration(
        border: Border(top: BorderSide(color: colors.line)),
      ),
      padding: const EdgeInsets.symmetric(horizontal: 16),
      child: Wrap(
        spacing: 18,
        runSpacing: 4,
        crossAxisAlignment: WrapCrossAlignment.center,
        children: [
          item(
            Transform.rotate(
              angle: math.pi / 4,
              child: Container(
                width: 9,
                height: 9,
                decoration: BoxDecoration(
                  color: colors.router,
                  borderRadius: BorderRadius.circular(1),
                ),
              ),
            ),
            l10n.taskTraceLegendRouter,
          ),
          item(swatch(Colors.blue), l10n.agentMatrixStatusRunning),
          item(swatch(Colors.green), l10n.agentMatrixStatusSuccess),
          item(swatch(Colors.red), l10n.agentMatrixStatusFailed),
          item(
            Container(width: 16, height: 1, color: colors.onSurfaceMuted),
            l10n.taskTraceLegendDependency,
          ),
        ],
      ),
    );
  }
}
