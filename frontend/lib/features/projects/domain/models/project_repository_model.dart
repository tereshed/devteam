import 'package:freezed_annotation/freezed_annotation.dart';
import 'package:frontend/features/projects/domain/models/project_model.dart';

part 'project_repository_model.freezed.dart';
part 'project_repository_model.g.dart';

/// Один git-репозиторий в составе проекта (мульти-репо).
///
/// Проект может содержать несколько репозиториев (например, отдельный под UI и под
/// высоконагруженную часть). [roleDescription] читает decomposer на бэкенде, чтобы
/// раскладывать подзадачи по нужному репо.
@freezed
abstract class ProjectRepositoryModel with _$ProjectRepositoryModel {
  const factory ProjectRepositoryModel({
    /// UUID репозитория
    required String id,

    /// UUID проекта-владельца
    @JsonKey(name: 'project_id')
    required String projectId,

    /// Короткий стабильный идентификатор репо (напр. 'ui', 'core', 'infra')
    required String slug,

    /// Человекочитаемое имя репозитория
    @JsonKey(name: 'display_name')
    required String displayName,

    /// Роль репо для decomposer (напр. «Flutter UI», «высоконагруженный Go-бэкенд»)
    @JsonKey(name: 'role_description')
    @Default('')
    String roleDescription,

    /// Тип провайдера: 'github', 'gitlab', 'bitbucket', 'local'
    @JsonKey(name: 'git_provider')
    required String gitProvider,

    /// URL Git-репозитория
    @JsonKey(name: 'git_url')
    required String gitUrl,

    /// Ветка по умолчанию
    @JsonKey(name: 'git_default_branch')
    @Default('main')
    String gitDefaultBranch,

    /// Git-кредентиал (краткие данные без секретов, может быть null)
    @JsonKey(name: 'git_credential')
    GitCredentialModel? gitCredential,

    /// Выбранный OAuth-аккаунт провайдера для этого репо (мульти-аккаунт).
    @JsonKey(name: 'git_integration_credential_id')
    String? gitIntegrationCredentialId,

    /// Имя вектор-коллекции (для Weaviate)
    @JsonKey(name: 'vector_collection')
    @Default('')
    String vectorCollection,

    /// Хэш последнего проиндексированного коммита
    @JsonKey(name: 'last_indexed_commit')
    @Default('')
    String lastIndexedCommit,

    /// Статус репо (как ProjectStatus): 'active', 'indexing', 'ready', 'indexing_failed', ...
    @Default('active')
    String status,

    /// Primary-репозиторий проекта (ровно один на проект)
    @JsonKey(name: 'is_primary')
    @Default(false)
    bool isPrimary,

    /// Порядок сортировки в UI
    @JsonKey(name: 'sort_order')
    @Default(0)
    int sortOrder,

    /// Дата создания
    @JsonKey(name: 'created_at')
    required DateTime createdAt,

    /// Дата последнего обновления
    @JsonKey(name: 'updated_at')
    required DateTime updatedAt,
  }) = _ProjectRepositoryModel;

  const ProjectRepositoryModel._();

  factory ProjectRepositoryModel.fromJson(Map<String, dynamic> json) =>
      _$ProjectRepositoryModelFromJson(json);
}
