import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/schedules/domain/cron_spec.dart';
import 'package:frontend/features/schedules/domain/models/scheduled_task_model.dart';
import 'package:frontend/features/schedules/presentation/controllers/schedules_controller.dart';
import 'package:frontend/features/team/data/team_providers.dart';
import 'package:frontend/l10n/app_localizations.dart';

/// Диалог создания / редактирования регулярной задачи.
///
/// Возвращает `true` через `Navigator.pop`, если расписание было сохранено.
class ScheduleFormDialog extends ConsumerStatefulWidget {
  const ScheduleFormDialog({
    super.key,
    required this.projectId,
    this.existing,
  });

  final String projectId;
  final ScheduledTaskModel? existing;

  @override
  ConsumerState<ScheduleFormDialog> createState() => _ScheduleFormDialogState();
}

class _ScheduleFormDialogState extends ConsumerState<ScheduleFormDialog> {
  final _formKey = GlobalKey<FormState>();
  late final TextEditingController _nameCtrl;
  late final TextEditingController _descCtrl;
  late final TextEditingController _cronCtrl;

  late ScheduleSpec _spec;
  String _priority = 'medium';
  String? _teamId;
  bool _isActive = true;
  bool _submitting = false;

  static const List<String> _priorities = ['low', 'medium', 'high', 'critical'];
  // cron dow: Вс=0, Пн..Сб=1..6. Порядок отображения — Пн..Вс.
  static const List<int> _weekOrder = [1, 2, 3, 4, 5, 6, 0];

  @override
  void initState() {
    super.initState();
    final e = widget.existing;
    _nameCtrl = TextEditingController(text: e?.name ?? '');
    _descCtrl = TextEditingController(text: e?.description ?? '');
    _spec = e != null
        ? ScheduleSpec.fromCron(e.cronExpression)
        : const ScheduleSpec(frequency: ScheduleFrequency.daily);
    _cronCtrl = TextEditingController(text: _spec.toCron());
    _priority = e?.priority ?? 'medium';
    _teamId = e?.teamId;
    _isActive = e?.isActive ?? true;
  }

  @override
  void dispose() {
    _nameCtrl.dispose();
    _descCtrl.dispose();
    _cronCtrl.dispose();
    super.dispose();
  }

  String _priorityLabel(AppLocalizations l10n, String p) {
    switch (p) {
      case 'critical':
        return l10n.taskPriorityCritical;
      case 'high':
        return l10n.taskPriorityHigh;
      case 'low':
        return l10n.taskPriorityLow;
      case 'medium':
      default:
        return l10n.taskPriorityMedium;
    }
  }

  String _weekdayLabel(AppLocalizations l10n, int dow) {
    switch (dow) {
      case 1:
        return l10n.weekdayShortMon;
      case 2:
        return l10n.weekdayShortTue;
      case 3:
        return l10n.weekdayShortWed;
      case 4:
        return l10n.weekdayShortThu;
      case 5:
        return l10n.weekdayShortFri;
      case 6:
        return l10n.weekdayShortSat;
      case 0:
      default:
        return l10n.weekdayShortSun;
    }
  }

  /// Итоговое cron-выражение из текущего состояния формы.
  String _effectiveCron() {
    if (_spec.frequency == ScheduleFrequency.custom) {
      return _cronCtrl.text.trim();
    }
    return _spec.toCron();
  }

  bool _isValidCron(String expr) {
    final parts = expr.trim().split(RegExp(r'\s+'));
    return parts.length == 5 && parts.every((p) => p.isNotEmpty);
  }

  Future<void> _pickTime() async {
    final picked = await showTimePicker(
      context: context,
      initialTime: _spec.timeOfDay,
    );
    if (picked != null) {
      setState(() => _spec = _spec.copyWith(timeOfDay: picked));
    }
  }

  Future<void> _onSubmit() async {
    final l10n = requireAppLocalizations(context, where: 'ScheduleFormDialog');
    if (!_formKey.currentState!.validate()) {
      return;
    }
    final cron = _effectiveCron();
    if (!_isValidCron(cron)) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(l10n.scheduleCronInvalid)),
      );
      return;
    }
    if (_spec.frequency == ScheduleFrequency.weekly && _spec.weekdays.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(l10n.scheduleWeekdaysRequired)),
      );
      return;
    }

    setState(() => _submitting = true);
    final controller =
        ref.read(schedulesControllerProvider(widget.projectId).notifier);
    try {
      if (widget.existing == null) {
        await controller.createSchedule(
          name: _nameCtrl.text.trim(),
          description: _descCtrl.text.trim(),
          cronExpression: cron,
          priority: _priority,
          teamId: _teamId,
          isActive: _isActive,
        );
      } else {
        await controller.updateSchedule(
          widget.existing!.id,
          name: _nameCtrl.text.trim(),
          description: _descCtrl.text.trim(),
          cronExpression: cron,
          priority: _priority,
          teamId: _teamId,
          clearTeam: _teamId == null,
          isActive: _isActive,
        );
      }
      if (!mounted) {
        return;
      }
      Navigator.of(context).pop(true);
    } catch (e) {
      if (!mounted) {
        return;
      }
      setState(() => _submitting = false);
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('$e')),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'ScheduleFormDialog');
    final teamsAsync = ref.watch(teamsProvider(widget.projectId));

    return AlertDialog(
      title: Text(
        widget.existing == null
            ? l10n.scheduleCreateTitle
            : l10n.scheduleEditTitle,
      ),
      content: SizedBox(
        width: 460,
        child: SingleChildScrollView(
          child: Form(
            key: _formKey,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                TextFormField(
                  controller: _nameCtrl,
                  decoration: InputDecoration(
                    labelText: l10n.scheduleNameLabel,
                    hintText: l10n.scheduleNameHint,
                  ),
                  maxLength: 500,
                  textInputAction: TextInputAction.next,
                  validator: (v) =>
                      (v == null || v.trim().isEmpty) ? l10n.scheduleNameRequired : null,
                ),
                const SizedBox(height: 8),
                TextFormField(
                  controller: _descCtrl,
                  decoration: InputDecoration(
                    labelText: l10n.scheduleDescriptionLabel,
                    hintText: l10n.scheduleDescriptionHint,
                    alignLabelWithHint: true,
                  ),
                  minLines: 3,
                  maxLines: 6,
                ),
                const SizedBox(height: 16),
                teamsAsync.when(
                  data: (teams) => DropdownButtonFormField<String?>(
                    initialValue:
                        teams.any((t) => t.id == _teamId) ? _teamId : null,
                    decoration: InputDecoration(labelText: l10n.scheduleTeamLabel),
                    items: [
                      DropdownMenuItem<String?>(
                        value: null,
                        child: Text(l10n.scheduleTeamNone),
                      ),
                      for (final t in teams)
                        DropdownMenuItem<String?>(
                          value: t.id,
                          child: Text(t.name),
                        ),
                    ],
                    onChanged: (v) => setState(() => _teamId = v),
                  ),
                  loading: () => const LinearProgressIndicator(),
                  error: (_, _) => DropdownButtonFormField<String?>(
                    initialValue: null,
                    decoration: InputDecoration(labelText: l10n.scheduleTeamLabel),
                    items: [
                      DropdownMenuItem<String?>(
                        value: null,
                        child: Text(l10n.scheduleTeamNone),
                      ),
                    ],
                    onChanged: (v) => setState(() => _teamId = v),
                  ),
                ),
                const SizedBox(height: 12),
                DropdownButtonFormField<String>(
                  initialValue: _priority,
                  decoration: InputDecoration(labelText: l10n.schedulePriorityLabel),
                  items: [
                    for (final p in _priorities)
                      DropdownMenuItem<String>(
                        value: p,
                        child: Text(_priorityLabel(l10n, p)),
                      ),
                  ],
                  onChanged: (v) => setState(() => _priority = v ?? 'medium'),
                ),
                const Divider(height: 32),
                DropdownButtonFormField<ScheduleFrequency>(
                  initialValue: _spec.frequency,
                  decoration: InputDecoration(labelText: l10n.scheduleFrequencyLabel),
                  items: [
                    DropdownMenuItem(
                      value: ScheduleFrequency.daily,
                      child: Text(l10n.scheduleFreqDaily),
                    ),
                    DropdownMenuItem(
                      value: ScheduleFrequency.weekly,
                      child: Text(l10n.scheduleFreqWeekly),
                    ),
                    DropdownMenuItem(
                      value: ScheduleFrequency.hourly,
                      child: Text(l10n.scheduleFreqHourly),
                    ),
                    DropdownMenuItem(
                      value: ScheduleFrequency.custom,
                      child: Text(l10n.scheduleFreqCustom),
                    ),
                  ],
                  onChanged: (v) => setState(() {
                    if (v != null) {
                      _spec = _spec.copyWith(frequency: v);
                    }
                  }),
                ),
                const SizedBox(height: 12),
                ..._buildFrequencyControls(l10n),
                const SizedBox(height: 16),
                _cronPreview(l10n),
              ],
            ),
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: _submitting ? null : () => Navigator.of(context).pop(false),
          child: Text(l10n.scheduleCancel),
        ),
        FilledButton(
          onPressed: _submitting ? null : _onSubmit,
          child: _submitting
              ? const SizedBox(
                  width: 18,
                  height: 18,
                  child: CircularProgressIndicator(strokeWidth: 2),
                )
              : Text(l10n.scheduleSave),
        ),
      ],
    );
  }

  List<Widget> _buildFrequencyControls(AppLocalizations l10n) {
    switch (_spec.frequency) {
      case ScheduleFrequency.daily:
        return [_timeRow(l10n)];
      case ScheduleFrequency.weekly:
        return [
          _timeRow(l10n),
          const SizedBox(height: 12),
          Text(l10n.scheduleWeekdaysLabel),
          const SizedBox(height: 6),
          Wrap(
            spacing: 6,
            children: [
              for (final dow in _weekOrder)
                FilterChip(
                  label: Text(_weekdayLabel(l10n, dow)),
                  selected: _spec.weekdays.contains(dow),
                  onSelected: (sel) => setState(() {
                    final next = {..._spec.weekdays};
                    if (sel) {
                      next.add(dow);
                    } else {
                      next.remove(dow);
                    }
                    _spec = _spec.copyWith(weekdays: next);
                  }),
                ),
            ],
          ),
        ];
      case ScheduleFrequency.hourly:
        return [
          Row(
            children: [
              Expanded(
                child: TextFormField(
                  initialValue: _spec.intervalHours.toString(),
                  decoration:
                      InputDecoration(labelText: l10n.scheduleIntervalHoursLabel),
                  keyboardType: TextInputType.number,
                  onChanged: (v) {
                    final n = int.tryParse(v);
                    if (n != null && n >= 1 && n <= 23) {
                      setState(() => _spec = _spec.copyWith(intervalHours: n));
                    }
                  },
                ),
              ),
            ],
          ),
        ];
      case ScheduleFrequency.custom:
        return [
          TextFormField(
            controller: _cronCtrl,
            decoration: InputDecoration(
              labelText: l10n.scheduleCronLabel,
              hintText: l10n.scheduleCronHint,
            ),
            onChanged: (_) => setState(() {}),
          ),
        ];
    }
  }

  Widget _timeRow(AppLocalizations l10n) {
    return Row(
      children: [
        Text('${l10n.scheduleTimeLabel}: '),
        const SizedBox(width: 8),
        OutlinedButton.icon(
          icon: const Icon(Icons.access_time, size: 18),
          onPressed: _pickTime,
          label: Text(_spec.timeOfDay.format(context)),
        ),
      ],
    );
  }

  Widget _cronPreview(AppLocalizations l10n) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Text(
        '${l10n.scheduleCronPreviewLabel}: ${_effectiveCron()}',
        style: Theme.of(context).textTheme.bodyMedium?.copyWith(
              fontFamily: 'monospace',
            ),
      ),
    );
  }
}
