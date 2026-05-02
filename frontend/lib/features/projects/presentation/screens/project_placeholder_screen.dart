import 'package:flutter/material.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';

/// Временная заглушка до [ProjectDashboardScreen] (задача 10.6).
class ProjectPlaceholderScreen extends StatelessWidget {
  const ProjectPlaceholderScreen({super.key, required this.projectId});

  final String projectId;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
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
        title: Text(l10n.projectsTitle),
      ),
      body: Center(child: Text(projectId)),
    );
  }
}
