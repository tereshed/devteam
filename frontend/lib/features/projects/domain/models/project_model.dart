import 'package:freezed_annotation/freezed_annotation.dart';

part 'project_model.freezed.dart';
part 'project_model.g.dart';

@freezed
abstract class ProjectModel with _$ProjectModel {
  const factory ProjectModel({
    /// UUID проекта
    required String id,

    /// Название проекта
    required String name,

    /// Описание проекта
    required String description,

    /// Тип провайдера: 'github', 'gitlab', 'bitbucket', 'local'
    @JsonKey(name: 'git_provider')
    required String gitProvider,

    /// HTTPS URL Git-репозитория
    @JsonKey(name: 'git_url')
    required String gitUrl,

    /// Ветка по умолчанию (например, 'main' или 'master')
    @JsonKey(name: 'git_default_branch')
    required String gitDefaultBranch,

    /// Git-кредентиал (краткие данные без секретов, может быть null)
    @JsonKey(name: 'git_credential')
    GitCredentialModel? gitCredential,

    /// Имя вектор-коллекции (для Weaviate)
    @JsonKey(name: 'vector_collection')
    required String vectorCollection,

    /// Технологический стек (JSON объект: {"backend": "Go", "frontend": "Flutter"})
    @JsonKey(name: 'tech_stack')
    @Default(<String, dynamic>{})
    Map<String, dynamic> techStack,

    /// Статус проекта: 'active', 'paused', 'archived', 'indexing', 'indexing_failed', 'ready'
    required String status,

    /// Дополнительные настройки (JSON объект)
    @Default(<String, dynamic>{})
    Map<String, dynamic> settings,

    /// Дата создания
    @JsonKey(name: 'created_at')
    required DateTime createdAt,

    /// Дата последнего обновления
    @JsonKey(name: 'updated_at')
    required DateTime updatedAt,
  }) = _ProjectModel;

  const ProjectModel._();

  factory ProjectModel.fromJson(Map<String, dynamic> json) =>
      _$ProjectModelFromJson(json);
}

@freezed
abstract class GitCredentialModel with _$GitCredentialModel {
  const factory GitCredentialModel({
    /// ID кредентиала
    required String id,

    /// Провайдер: 'github' | 'gitlab' | 'bitbucket'
    required String provider,

    /// Тип аутентификации: 'token' | 'ssh_key' | 'oauth'
    @JsonKey(name: 'auth_type')
    required String authType,

    /// Метка для UI
    required String label,
  }) = _GitCredentialModel;

  const GitCredentialModel._();

  factory GitCredentialModel.fromJson(Map<String, dynamic> json) =>
      _$GitCredentialModelFromJson(json);
}

/// Статусы проекта (ProjectStatus в backend)
const projectStatuses = [
  'active',           // Активный проект
  'paused',           // Приостановленный
  'archived',         // Архивированный
  'indexing',         // Идёт индексация кода
  'indexing_failed',  // Ошибка индексации
  'ready',            // Готов к работе
];

/// Git-провайдеры (GitProvider в backend)
const gitProviders = [
  'github',
  'gitlab',
  'bitbucket',
  'local',  // Локальный git (без API провайдера)
];
