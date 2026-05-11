import 'package:flutter/material.dart';

/// Одна строка key/value для секции tech stack на экране настроек проекта (13.4).
class ProjectSettingsTechFieldRow {
  ProjectSettingsTechFieldRow({String keyText = '', String valueText = ''})
      : keyCtrl = TextEditingController(text: keyText),
        valueCtrl = TextEditingController(text: valueText);

  final TextEditingController keyCtrl;
  final TextEditingController valueCtrl;

  void dispose() {
    keyCtrl.dispose();
    valueCtrl.dispose();
  }
}
