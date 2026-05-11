import 'package:frontend/features/projects/domain/models/project_model.dart';
import 'package:frontend/features/projects/domain/requests.dart';
import 'package:meta/meta.dart';

/// Строковые значения [ProjectModel.techStack] для сравнения с редактором (MVP 13.4).
Map<String, String> projectBaselineTechStackStrings(ProjectModel baseline) {
  final out = <String, String>{};
  for (final e in baseline.techStack.entries) {
    final v = e.value;
    if (v is String) {
      out[e.key] = v;
    }
  }
  return out;
}

bool _stringMapsEqual(Map<String, String> a, Map<String, String> b) {
  if (a.length != b.length) {
    return false;
  }
  for (final e in a.entries) {
    if (b[e.key] != e.value) {
      return false;
    }
  }
  return true;
}

/// Поверхностное сравнение jsonb tech_stack (один уровень ключей; MVP 13.4).
bool _shallowTechStackEquals(Map<String, dynamic> a, Map<String, dynamic> b) {
  if (a.length != b.length) {
    return false;
  }
  for (final e in a.entries) {
    if (b[e.key] != e.value) {
      return false;
    }
  }
  return true;
}

/// Инвариант PATCH: нельзя одновременно слать `tech_stack` и `clear_tech_stack: true`.
@visibleForTesting
void ensureTechStackMutuallyExclusive({
  Map<String, dynamic>? techStack,
  bool? clearTechStack,
}) {
  if (techStack != null && clearTechStack == true) {
    throw StateError('tech_stack and clear_tech_stack must not be combined');
  }
}

/// Dirty-патч для [PUT /projects/:id] с экрана настроек (13.4). `null` — сохранять нечего.
///
/// Инварианты: не смешивает [UpdateProjectRequest.techStack] и [UpdateProjectRequest.clearTechStack];
/// при смене провайдера на [kLocalGitProvider] при наличии [ProjectModel.gitCredential] в базовой
/// модели добавляет [UpdateProjectRequest.removeGitCredential].
UpdateProjectRequest? buildProjectSettingsUpdateRequest({
  required ProjectModel baseline,
  required String gitProvider,
  required String gitUrl,
  required String gitDefaultBranch,
  required String vectorCollection,
  required Map<String, String> techStackEditedNonEmptyKeys,
  required bool pendingRemoveGitCredential,
  required bool explicitClearTechStack,
}) {
  String? gitProviderOut;
  if (gitProvider != baseline.gitProvider) {
    gitProviderOut = gitProvider;
  }

  final trimmedUrl = gitUrl.trim();
  String? gitUrlOut;
  if (trimmedUrl != baseline.gitUrl.trim()) {
    gitUrlOut = trimmedUrl;
  }

  String? gitBranchOut;
  final trimmedBranch = gitDefaultBranch.trim();
  if (trimmedBranch != baseline.gitDefaultBranch.trim()) {
    gitBranchOut = trimmedBranch;
  }

  String? vectorOut;
  final trimmedVector = vectorCollection.trim();
  if (trimmedVector != baseline.vectorCollection.trim()) {
    vectorOut = trimmedVector;
  }

  bool? removeGitCredential;
  if (pendingRemoveGitCredential) {
    removeGitCredential = true;
  }
  if (gitProviderOut == kLocalGitProvider &&
      baseline.gitCredential != null &&
      baseline.gitProvider != kLocalGitProvider) {
    removeGitCredential = true;
  }

  final baselineTech = projectBaselineTechStackStrings(baseline);
  Map<String, dynamic>? techStackOut;
  bool? clearTechStack;

  if (techStackEditedNonEmptyKeys.isNotEmpty) {
    if (!_stringMapsEqual(techStackEditedNonEmptyKeys, baselineTech)) {
      // Сохраняем non-string ключи baseline (например `version: 18`), подменяем/удаляем только строковые пары из UI.
      final merged = Map<String, dynamic>.from(baseline.techStack);
      for (final k in baselineTech.keys) {
        if (!techStackEditedNonEmptyKeys.containsKey(k)) {
          merged.remove(k);
        }
      }
      for (final e in techStackEditedNonEmptyKeys.entries) {
        merged[e.key] = e.value;
      }
      if (!_shallowTechStackEquals(merged, baseline.techStack)) {
        techStackOut = merged;
      }
    }
  } else if (explicitClearTechStack || baselineTech.isNotEmpty) {
    clearTechStack = true;
  }

  ensureTechStackMutuallyExclusive(
    techStack: techStackOut,
    clearTechStack: clearTechStack,
  );

  final req = UpdateProjectRequest(
    gitProvider: gitProviderOut,
    gitUrl: gitUrlOut,
    gitDefaultBranch: gitBranchOut,
    vectorCollection: vectorOut,
    techStack: techStackOut,
    clearTechStack: clearTechStack,
    removeGitCredential: removeGitCredential,
  );

  if (req.toJson().isEmpty) {
    return null;
  }
  return req;
}

/// Клиентская валидация имени коллекции Weaviate (13.4).
///
/// **Продуктовый выбор:** пустая строка после [trim] разрешена (как пустой `vector_collection` при создании
/// проекта в 10.5); непустое значение — строгий regex (`^[A-Z][A-Za-z0-9_]*$`).
bool isValidVectorCollectionName(String trimmed) {
  if (trimmed.isEmpty) {
    return true;
  }
  return RegExp(r'^[A-Z][A-Za-z0-9_]*$').hasMatch(trimmed);
}
