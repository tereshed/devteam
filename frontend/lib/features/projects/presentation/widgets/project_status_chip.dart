import 'package:flutter/material.dart';
import 'package:frontend/features/projects/presentation/utils/project_status_display.dart';

class ProjectStatusChip extends StatelessWidget {
  const ProjectStatusChip({required this.status, super.key});
  final String status;

  @override
  Widget build(BuildContext context) {
    final d = projectStatusDisplay(context, status);
    return Chip(
      avatar: Icon(d.icon, size: 14, color: d.color),
      label: Text(d.label, style: TextStyle(color: d.color, fontSize: 12)),
      backgroundColor: d.color.withValues(alpha: 0.1),
      side: BorderSide(color: d.color.withValues(alpha: 0.3)),
      padding: const EdgeInsets.symmetric(horizontal: 4),
      visualDensity: VisualDensity.compact,
    );
  }
}
