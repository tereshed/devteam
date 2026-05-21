import 'package:freezed_annotation/freezed_annotation.dart';

part 'git_repository_model.freezed.dart';
part 'git_repository_model.g.dart';

@freezed
abstract class GitRepositoryModel with _$GitRepositoryModel {
  const factory GitRepositoryModel({
    required String name,
    @JsonKey(name: 'full_name') required String fullName,
    @JsonKey(name: 'html_url') required String htmlUrl,
    @JsonKey(name: 'clone_url') required String cloneUrl,
    String? description,
  }) = _GitRepositoryModel;

  factory GitRepositoryModel.fromJson(Map<String, dynamic> json) =>
      _$GitRepositoryModelFromJson(json);
}
