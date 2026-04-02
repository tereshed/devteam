import 'package:flutter/foundation.dart';

/// Модель Prompt
@immutable
class Prompt {
  final String id;
  final String name;
  final String description;
  final String template;
  final Map<String, dynamic>? jsonSchema;
  final bool isActive;
  final DateTime createdAt;
  final DateTime updatedAt;

  const Prompt({
    required this.id,
    required this.name,
    required this.description,
    required this.template,
    this.jsonSchema,
    required this.isActive,
    required this.createdAt,
    required this.updatedAt,
  });

  factory Prompt.fromJson(Map<String, dynamic> json) {
    return Prompt(
      id: json['id'] as String,
      name: json['name'] as String,
      description: json['description'] as String? ?? '',
      template: json['template'] as String,
      jsonSchema: json['json_schema'] as Map<String, dynamic>?,
      isActive: json['is_active'] as bool,
      createdAt: DateTime.parse(json['created_at'] as String),
      updatedAt: DateTime.parse(json['updated_at'] as String),
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'name': name,
      'description': description,
      'template': template,
      'json_schema': jsonSchema,
      'is_active': isActive,
      'created_at': createdAt.toIso8601String(),
      'updated_at': updatedAt.toIso8601String(),
    };
  }
}
