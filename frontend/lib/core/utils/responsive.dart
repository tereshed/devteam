import 'package:flutter/material.dart';

/// Breakpoints для адаптивной верстки
class Breakpoints {
  static const double mobile = 600;
  static const double tablet = 1200;
}

/// Утилиты для адаптивной верстки
class Responsive {
  /// Определяет тип устройства на основе ширины экрана
  static DeviceType getDeviceType(BuildContext context) {
    final width = MediaQuery.of(context).size.width;

    if (width < Breakpoints.mobile) {
      return DeviceType.mobile;
    } else if (width < Breakpoints.tablet) {
      return DeviceType.tablet;
    } else {
      return DeviceType.desktop;
    }
  }

  /// Проверяет, является ли устройство мобильным
  static bool isMobile(BuildContext context) {
    return getDeviceType(context) == DeviceType.mobile;
  }

  /// Проверяет, является ли устройство планшетом
  static bool isTablet(BuildContext context) {
    return getDeviceType(context) == DeviceType.tablet;
  }

  /// Проверяет, является ли устройство десктопом
  static bool isDesktop(BuildContext context) {
    return getDeviceType(context) == DeviceType.desktop;
  }

  /// Возвращает адаптивный padding на основе типа устройства
  static EdgeInsets getPadding(BuildContext context) {
    final deviceType = getDeviceType(context);

    switch (deviceType) {
      case DeviceType.mobile:
        return const EdgeInsets.all(24.0);
      case DeviceType.tablet:
        return const EdgeInsets.symmetric(horizontal: 48.0, vertical: 32.0);
      case DeviceType.desktop:
        return const EdgeInsets.symmetric(horizontal: 120.0, vertical: 48.0);
    }
  }

  /// Возвращает максимальную ширину контента для центрирования на больших экранах
  static double? getMaxContentWidth(BuildContext context) {
    final deviceType = getDeviceType(context);

    switch (deviceType) {
      case DeviceType.mobile:
        return null; // На мобильных используем всю ширину
      case DeviceType.tablet:
        return 600; // На планшетах ограничиваем ширину формы
      case DeviceType.desktop:
        return 500; // На десктопе также ограничиваем для удобства чтения
    }
  }

  /// Возвращает адаптивный размер шрифта
  static double getFontSize(
    BuildContext context, {
    required double mobile,
    required double tablet,
    required double desktop,
  }) {
    final deviceType = getDeviceType(context);

    switch (deviceType) {
      case DeviceType.mobile:
        return mobile;
      case DeviceType.tablet:
        return tablet;
      case DeviceType.desktop:
        return desktop;
    }
  }
}

/// Тип устройства
enum DeviceType {
  mobile, // < 600 dp
  tablet, // 600 dp - 1200 dp
  desktop, // > 1200 dp
}

/// Адаптивные отступы на основе размера экрана
class Spacing {
  /// Мини отступ (8/12/16)
  static double mini(BuildContext context) {
    final deviceType = Responsive.getDeviceType(context);
    switch (deviceType) {
      case DeviceType.mobile:
        return 8.0;
      case DeviceType.tablet:
        return 12.0;
      case DeviceType.desktop:
        return 16.0;
    }
  }

  /// Малый отступ (16/20/24)
  static double small(BuildContext context) {
    final deviceType = Responsive.getDeviceType(context);
    switch (deviceType) {
      case DeviceType.mobile:
        return 16.0;
      case DeviceType.tablet:
        return 20.0;
      case DeviceType.desktop:
        return 24.0;
    }
  }

  /// Средний отступ (24/32/40)
  static double medium(BuildContext context) {
    final deviceType = Responsive.getDeviceType(context);
    switch (deviceType) {
      case DeviceType.mobile:
        return 24.0;
      case DeviceType.tablet:
        return 32.0;
      case DeviceType.desktop:
        return 40.0;
    }
  }

  /// Большой отступ (32/48/56)
  static double large(BuildContext context) {
    final deviceType = Responsive.getDeviceType(context);
    switch (deviceType) {
      case DeviceType.mobile:
        return 32.0;
      case DeviceType.tablet:
        return 48.0;
      case DeviceType.desktop:
        return 56.0;
    }
  }

  /// Очень большой отступ (48/64/80)
  static double xLarge(BuildContext context) {
    final deviceType = Responsive.getDeviceType(context);
    switch (deviceType) {
      case DeviceType.mobile:
        return 48.0;
      case DeviceType.tablet:
        return 64.0;
      case DeviceType.desktop:
        return 80.0;
    }
  }

  /// Адаптивный padding для Card
  static EdgeInsets cardPadding(BuildContext context) {
    final deviceType = Responsive.getDeviceType(context);
    switch (deviceType) {
      case DeviceType.mobile:
        return const EdgeInsets.all(16.0);
      case DeviceType.tablet:
        return const EdgeInsets.all(20.0);
      case DeviceType.desktop:
        return const EdgeInsets.all(24.0);
    }
  }

  /// Адаптивный padding для кнопок (вертикальный)
  static EdgeInsets buttonPadding(BuildContext context) {
    final deviceType = Responsive.getDeviceType(context);
    switch (deviceType) {
      case DeviceType.mobile:
        return const EdgeInsets.symmetric(vertical: 16.0);
      case DeviceType.tablet:
        return const EdgeInsets.symmetric(vertical: 18.0);
      case DeviceType.desktop:
        return const EdgeInsets.symmetric(vertical: 20.0);
    }
  }

  /// Адаптивный размер иконки
  static double iconSize(BuildContext context) {
    final deviceType = Responsive.getDeviceType(context);
    switch (deviceType) {
      case DeviceType.mobile:
        return 20.0;
      case DeviceType.tablet:
        return 22.0;
      case DeviceType.desktop:
        return 24.0;
    }
  }

  /// Адаптивный радиус аватара
  static double avatarRadius(BuildContext context) {
    final deviceType = Responsive.getDeviceType(context);
    switch (deviceType) {
      case DeviceType.mobile:
        return 50.0;
      case DeviceType.tablet:
        return 60.0;
      case DeviceType.desktop:
        return 70.0;
    }
  }
}
