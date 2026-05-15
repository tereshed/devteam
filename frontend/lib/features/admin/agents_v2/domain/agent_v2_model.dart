import 'package:flutter/foundation.dart';

@immutable
class AgentV2 {
  final String id;
  final String name;
  final String role;
  final String roleDescription;
  final String executionKind; // 'llm' | 'sandbox'
  final String? systemPrompt; // не включается в List; присутствует в Get
  final String? model;
  final double? temperature;
  final int? maxTokens;
  final String? codeBackend;
  final bool isActive;
  final DateTime createdAt;
  final DateTime updatedAt;

  const AgentV2({
    required this.id,
    required this.name,
    required this.role,
    required this.roleDescription,
    required this.executionKind,
    required this.isActive,
    required this.createdAt,
    required this.updatedAt,
    this.systemPrompt,
    this.model,
    this.temperature,
    this.maxTokens,
    this.codeBackend,
  });

  bool get isLlm => executionKind == 'llm';
  bool get isSandbox => executionKind == 'sandbox';

  factory AgentV2.fromJson(Map<String, dynamic> json) {
    final tempRaw = json['temperature'];
    final double? temp = tempRaw == null
        ? null
        : (tempRaw is int ? tempRaw.toDouble() : (tempRaw as num).toDouble());
    return AgentV2(
      id: json['id'] as String,
      name: json['name'] as String,
      role: json['role'] as String? ?? '',
      roleDescription: json['role_description'] as String? ?? '',
      executionKind: json['execution_kind'] as String,
      systemPrompt: json['system_prompt'] as String?,
      model: json['model'] as String?,
      temperature: temp,
      maxTokens: json['max_tokens'] as int?,
      codeBackend: json['code_backend'] as String?,
      isActive: json['is_active'] as bool? ?? true,
      createdAt: DateTime.parse(json['created_at'] as String),
      updatedAt: DateTime.parse(json['updated_at'] as String),
    );
  }
}

@immutable
class AgentV2Page {
  final int total;
  final List<AgentV2> items;
  final int limit;
  final int offset;

  const AgentV2Page({
    required this.total,
    required this.items,
    required this.limit,
    required this.offset,
  });

  factory AgentV2Page.fromJson(Map<String, dynamic> json) {
    final raw = json['items'];
    final items = raw is List
        ? raw
            .whereType<Map<String, dynamic>>()
            .map(AgentV2.fromJson)
            .toList(growable: false)
        : <AgentV2>[];
    return AgentV2Page(
      total: (json['total'] as num?)?.toInt() ?? items.length,
      items: items,
      limit: (json['limit'] as num?)?.toInt() ?? items.length,
      offset: (json['offset'] as num?)?.toInt() ?? 0,
    );
  }
}

@immutable
class AgentV2SecretRef {
  final String id;
  final String keyName;
  final DateTime createdAt;

  const AgentV2SecretRef({
    required this.id,
    required this.keyName,
    required this.createdAt,
  });

  factory AgentV2SecretRef.fromJson(Map<String, dynamic> json) {
    return AgentV2SecretRef(
      id: json['id'] as String,
      keyName: json['key_name'] as String,
      createdAt: DateTime.parse(json['created_at'] as String),
    );
  }
}
