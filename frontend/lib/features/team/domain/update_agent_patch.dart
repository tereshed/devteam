import 'package:frontend/core/json/patch.dart';

/// Тело `PATCH /projects/:id/team/agents/:agentId` (зеркало Go `PatchAgentRequest`).
class UpdateAgentPatch {
  const UpdateAgentPatch({
    this.model = const Patch<String>.omit(),
    this.promptId = const Patch<String?>.omit(),
    this.codeBackend = const Patch<String?>.omit(),
    this.isActive = const Patch<bool>.omit(),
  });

  final Patch<String> model;
  final Patch<String?> promptId;
  final Patch<String?> codeBackend;
  final Patch<bool> isActive;

  Map<String, dynamic> toWireJson() {
    final m = <String, dynamic>{};
    if (model.isClear) {
      m['model'] = null;
    } else if (model.isValue) {
      m['model'] = model.requireValue;
    }
    if (promptId.isClear) {
      m['prompt_id'] = null;
    } else if (promptId.isValue) {
      m['prompt_id'] = promptId.requireValue;
    }
    if (codeBackend.isClear) {
      m['code_backend'] = null;
    } else if (codeBackend.isValue) {
      m['code_backend'] = codeBackend.requireValue;
    }
    if (isActive.isValue) {
      m['is_active'] = isActive.requireValue;
    }
    return m;
  }
}
