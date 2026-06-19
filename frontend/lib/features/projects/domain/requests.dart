import 'package:freezed_annotation/freezed_annotation.dart';
import 'package:frontend/features/projects/domain/models/project_model.dart';
import 'package:frontend/features/projects/domain/models/project_repository_model.dart';

part 'requests.freezed.dart';
part 'requests.g.dart';

/// Фильтр для списка проектов
@freezed
abstract class ProjectListFilter with _$ProjectListFilter {
  const factory ProjectListFilter({
    String? status,
    String? gitProvider,
    String? search,
    @Default('created_at') String orderBy,
    @Default('DESC') String orderDir,
  }) = _ProjectListFilter;

  const ProjectListFilter._();

  /// Преобразует фильтр в query parameters для Dio GET запроса
  Map<String, dynamic> toQueryParameters() {
    return {
      if (status != null) 'status': status,
      if (gitProvider != null) 'git_provider': gitProvider,
      if (search != null && search!.isNotEmpty) 'search': search,
      'order_by': orderBy,
      'order_dir': orderDir,
    };
  }
}

/// Ответ на запрос списка проектов
@freezed
abstract class ProjectListResponse with _$ProjectListResponse {
  const factory ProjectListResponse({
    @Default([]) List<ProjectModel> projects,
    @Default(0) int total,
    @Default(20) int limit,
    @Default(0) int offset,
  }) = _ProjectListResponse;

  const ProjectListResponse._();

  factory ProjectListResponse.fromJson(Map<String, dynamic> json) =>
      _$ProjectListResponseFromJson(json);
}

/// Request для создания проекта
@freezed
abstract class CreateProjectRequest with _$CreateProjectRequest {
  const factory CreateProjectRequest({
    required String name,
    @Default('') String description,
    @JsonKey(name: 'git_provider')
    required String gitProvider,
    @JsonKey(name: 'git_url')
    required String gitUrl,
    @JsonKey(name: 'git_default_branch')
    @Default('main')
    String gitDefaultBranch,
    @JsonKey(name: 'git_credential_id')
    String? gitCredentialId,
    @JsonKey(name: 'git_integration_credential_id')
    String? gitIntegrationCredentialId,
    @JsonKey(name: 'vector_collection')
    required String vectorCollection,
    @JsonKey(name: 'tech_stack')
    @Default({})
    Map<String, dynamic> techStack,
    @Default('active') String status,
    @Default({}) Map<String, dynamic> settings,
  }) = _CreateProjectRequest;

  const CreateProjectRequest._();

  factory CreateProjectRequest.fromJson(Map<String, dynamic> json) =>
      _$CreateProjectRequestFromJson(json);
}

/// Request для обновления проекта (partial update)
@freezed
abstract class UpdateProjectRequest with _$UpdateProjectRequest {
  @JsonSerializable(includeIfNull: false)
  const factory UpdateProjectRequest({
    String? name,
    String? description,
    @JsonKey(name: 'git_provider')
    String? gitProvider,
    @JsonKey(name: 'git_url')
    String? gitUrl,
    @JsonKey(name: 'git_default_branch')
    String? gitDefaultBranch,
    @JsonKey(name: 'git_credential_id')
    String? gitCredentialId,
    @JsonKey(name: 'git_integration_credential_id')
    String? gitIntegrationCredentialId,
    @JsonKey(name: 'remove_git_integration_credential')
    bool? removeGitIntegrationCredential,
    @JsonKey(name: 'vector_collection')
    String? vectorCollection,
    String? status,
    @JsonKey(name: 'tech_stack')
    Map<String, dynamic>? techStack,
    Map<String, dynamic>? settings,
    /// Только `true` попадает в JSON ([includeIfNull: false]); иначе ключ отсутствует.
    @JsonKey(name: 'remove_git_credential')
    bool? removeGitCredential,
    @JsonKey(name: 'clear_tech_stack')
    bool? clearTechStack,
    @JsonKey(name: 'clear_settings')
    bool? clearSettings,
    /// Промпт ассистента проекта. null (includeIfNull:false) — не менять;
    /// пустая строка "" — сброс к user-промпту.
    @JsonKey(name: 'assistant_prompt')
    String? assistantPrompt,

    /// Шаблон имён веток. null — не менять; "" — сброс к дефолту.
    @JsonKey(name: 'branch_name_template')
    String? branchNameTemplate,

    /// Явный regex формата ветки. null — не менять; "" — сброс к выведенному.
    @JsonKey(name: 'branch_name_pattern')
    String? branchNamePattern,

    /// Запрет ручного override имени ветки.
    @JsonKey(name: 'branch_naming_locked')
    bool? branchNamingLocked,
  }) = _UpdateProjectRequest;

  const UpdateProjectRequest._();

  factory UpdateProjectRequest.fromJson(Map<String, dynamic> json) =>
      _$UpdateProjectRequestFromJson(json);
}

/// Ответ на запрос списка репозиториев проекта (мульти-репо)
@freezed
abstract class RepositoryListResponse with _$RepositoryListResponse {
  const factory RepositoryListResponse({
    @Default(<ProjectRepositoryModel>[]) List<ProjectRepositoryModel> repositories,
    @Default(0) int total,
  }) = _RepositoryListResponse;

  const RepositoryListResponse._();

  factory RepositoryListResponse.fromJson(Map<String, dynamic> json) =>
      _$RepositoryListResponseFromJson(json);
}

/// Request для добавления репозитория в проект
@freezed
abstract class CreateRepositoryRequest with _$CreateRepositoryRequest {
  const factory CreateRepositoryRequest({
    required String slug,
    @JsonKey(name: 'display_name')
    required String displayName,
    @JsonKey(name: 'role_description')
    @Default('')
    String roleDescription,
    @JsonKey(name: 'git_provider')
    required String gitProvider,
    @JsonKey(name: 'git_url')
    required String gitUrl,
    @JsonKey(name: 'git_default_branch')
    @Default('main')
    String gitDefaultBranch,
    @JsonKey(name: 'git_credential_id')
    String? gitCredentialId,
    @JsonKey(name: 'git_integration_credential_id')
    String? gitIntegrationCredentialId,
    @JsonKey(name: 'is_primary')
    @Default(false)
    bool isPrimary,
    @JsonKey(name: 'sort_order')
    @Default(0)
    int sortOrder,
  }) = _CreateRepositoryRequest;

  const CreateRepositoryRequest._();

  factory CreateRepositoryRequest.fromJson(Map<String, dynamic> json) =>
      _$CreateRepositoryRequestFromJson(json);
}

/// Request для обновления репозитория проекта (partial update)
@freezed
abstract class UpdateRepositoryRequest with _$UpdateRepositoryRequest {
  @JsonSerializable(includeIfNull: false)
  const factory UpdateRepositoryRequest({
    @JsonKey(name: 'display_name')
    String? displayName,
    @JsonKey(name: 'role_description')
    String? roleDescription,
    @JsonKey(name: 'git_provider')
    String? gitProvider,
    @JsonKey(name: 'git_url')
    String? gitUrl,
    @JsonKey(name: 'git_default_branch')
    String? gitDefaultBranch,
    @JsonKey(name: 'git_credential_id')
    String? gitCredentialId,
    @JsonKey(name: 'git_integration_credential_id')
    String? gitIntegrationCredentialId,
    @JsonKey(name: 'remove_git_integration_credential')
    bool? removeGitIntegrationCredential,
    @JsonKey(name: 'is_primary')
    bool? isPrimary,
    @JsonKey(name: 'sort_order')
    int? sortOrder,
  }) = _UpdateRepositoryRequest;

  const UpdateRepositoryRequest._();

  factory UpdateRepositoryRequest.fromJson(Map<String, dynamic> json) =>
      _$UpdateRepositoryRequestFromJson(json);
}
