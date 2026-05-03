import 'package:flutter/material.dart';

/// Заглушка раздела дашборда проекта до Sprint 11–12.
class ProjectDestinationPlaceholder extends StatelessWidget {
  const ProjectDestinationPlaceholder({
    super.key,
    required this.title,
  });

  final String title;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Text(
        title,
        style: Theme.of(context).textTheme.titleLarge,
        textAlign: TextAlign.center,
      ),
    );
  }
}
