import 'package:freezed_annotation/freezed_annotation.dart';

part 'repo_env_file_model.freezed.dart';
part 'repo_env_file_model.g.dart';

/// «Инъекция env-файла» уровня репозитория проекта (метаданные).
///
/// Один файл на репозиторий: перед запуском агента sandbox пишет содержимое в файл
/// [fileName] внутри папки [targetDir] (пусто — корень репо) рабочей копии репозитория
/// и исключает его из git (не попадает в diff/commit/push). Содержимое хранится на
/// бэкенде в зашифрованном виде и НЕ возвращается (write-only): редактирование —
/// это полная перезапись файла.
@freezed
abstract class RepoEnvFileModel with _$RepoEnvFileModel {
  const factory RepoEnvFileModel({
    /// UUID записи
    required String id,

    /// UUID репозитория проекта (project_repositories.id)
    @JsonKey(name: 'project_repository_id')
    required String projectRepositoryId,

    /// Имя создаваемого файла, например `.env`
    @JsonKey(name: 'file_name')
    required String fileName,

    /// Относительная папка внутри репо (пусто — корень)
    @JsonKey(name: 'target_dir')
    @Default('')
    String targetDir,

    @JsonKey(name: 'created_at')
    String? createdAt,

    @JsonKey(name: 'updated_at')
    String? updatedAt,
  }) = _RepoEnvFileModel;

  factory RepoEnvFileModel.fromJson(Map<String, dynamic> json) =>
      _$RepoEnvFileModelFromJson(json);
}
