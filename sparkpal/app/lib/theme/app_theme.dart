import 'package:flutter/material.dart';

/// 伙伴设计系统。
///
/// 配色用企业风中性灰阶；结构（圆角 / 字号 / 间距）沿用星语。
/// 深浅两套 token 同源，切换只换一组常量。
class Sp {
  Sp._();

  static bool dark = false;

  /// 手动切换：system / light / dark
  static ThemeMode mode = ThemeMode.system;

  /* ── 浅色 ── */
  static const _lBg = Color(0xFFF5F6F8);
  static const _lCard = Color(0xFFFFFFFF);
  static const _lBorder = Color(0xFFE4E7EC);
  static const _lBorderCtl = Color(0xFFD0D5DD);
  static const _lText = Color(0xFF101828);
  static const _lBody = Color(0xFF344054);
  static const _lSub = Color(0xFF667085);
  static const _lFaint = Color(0xFF98A2B3);
  static const _lBtn = Color(0xFF101828);
  static const _lSpark = Color(0xFFC2410C);
  static const _lOk = Color(0xFF067647);

  /* ── 深色 ── */
  static const _dBg = Color(0xFF0C111D);
  static const _dCard = Color(0xFF161B26);
  static const _dBorder = Color(0xFF262B36);
  static const _dBorderCtl = Color(0xFF333945);
  static const _dText = Color(0xFFF5F6F7);
  static const _dBody = Color(0xFFCECFD2);
  static const _dSub = Color(0xFF94969C);
  static const _dFaint = Color(0xFF6B6F76);
  static const _dBtn = Color(0xFFF5F6F7);
  static const _dSpark = Color(0xFFF79009);
  static const _dOk = Color(0xFF75E0A7);

  /* ── 派生 ── */
  static Color get bg => dark ? _dBg : _lBg;
  static Color get card => dark ? _dCard : _lCard;
  static Color get border => dark ? _dBorder : _lBorder;
  static Color get borderCtl => dark ? _dBorderCtl : _lBorderCtl;
  static Color get text => dark ? _dText : _lText;
  static Color get body => dark ? _dBody : _lBody;
  static Color get sub => dark ? _dSub : _lSub;
  static Color get faint => dark ? _dFaint : _lFaint;
  static Color get btnBg => dark ? _dBtn : _lBtn;
  static Color get btnFg => dark ? _dBg : Colors.white;
  static Color get spark => dark ? _dSpark : _lSpark;
  static Color get ok => dark ? _dOk : _lOk;

  /* ── 圆角（星语 rCard / rInput / rChip）── */
  static const double rCard = 8;
  static const double rInput = 10;
  static const double rChip = 7;

  /* ── 字号（星语 fsTitle…fsTiny）── */
  static const double fsTitle = 22;
  static const double fsName = 15;
  static const double fsBody = 14.5;
  static const double fsItem = 14;
  static const double fsSub = 12.5;
  static const double fsSmall = 11.5;
  static const double fsTiny = 10.5;

  /* ── 间距（4/8 网格）── */
  static const double s1 = 4;
  static const double s2 = 8;
  static const double s3 = 12;
  static const double s4 = 16;
  static const double s6 = 24;
  static const double s8 = 32;

  static ThemeData theme(bool isDark) {
    dark = isDark;
    final scheme = ColorScheme(
      brightness: isDark ? Brightness.dark : Brightness.light,
      primary: btnBg,
      onPrimary: btnFg,
      secondary: spark,
      onSecondary: isDark ? Colors.black : Colors.white,
      error: const Color(0xFFB42318),
      onError: Colors.white,
      surface: card,
      onSurface: text,
    );

    final base = isDark ? ThemeData.dark() : ThemeData.light();

    return base.copyWith(
      colorScheme: scheme,
      scaffoldBackgroundColor: bg,
      canvasColor: bg,
      dividerColor: border,
      // 字体：不指定 fontFamily，交给系统（iOS 用 PingFang，安卓用厂商字体）
      textTheme: base.textTheme.apply(
        bodyColor: text,
        displayColor: text,
      ),
      appBarTheme: AppBarTheme(
        backgroundColor: bg,
        surfaceTintColor: Colors.transparent,
        elevation: 0,
        centerTitle: false,
        iconTheme: IconThemeData(color: body, size: 20),
        titleTextStyle: TextStyle(
          color: text,
          fontSize: fsName,
          fontWeight: FontWeight.w600,
        ),
      ),
      cardTheme: CardThemeData(
        color: card,
        elevation: 0,
        margin: EdgeInsets.zero,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(rCard),
          side: BorderSide(color: border),
        ),
      ),
      filledButtonTheme: FilledButtonThemeData(
        style: FilledButton.styleFrom(
          backgroundColor: btnBg,
          foregroundColor: btnFg,
          minimumSize: const Size.fromHeight(48),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(rInput),
          ),
          textStyle: const TextStyle(fontSize: fsItem, fontWeight: FontWeight.w600),
        ),
      ),
      outlinedButtonTheme: OutlinedButtonThemeData(
        style: OutlinedButton.styleFrom(
          backgroundColor: card,
          foregroundColor: body,
          minimumSize: const Size.fromHeight(48),
          side: BorderSide(color: borderCtl),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(rInput),
          ),
          textStyle: const TextStyle(fontSize: fsItem, fontWeight: FontWeight.w600),
        ),
      ),
      inputDecorationTheme: InputDecorationTheme(
        filled: true,
        fillColor: card,
        hintStyle: TextStyle(color: faint, fontSize: fsBody),
        contentPadding: const EdgeInsets.symmetric(horizontal: s3, vertical: s3),
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(rInput),
          borderSide: BorderSide(color: borderCtl),
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(rInput),
          borderSide: BorderSide(color: borderCtl),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(rInput),
          borderSide: BorderSide(color: text, width: 1.4),
        ),
      ),
      snackBarTheme: SnackBarThemeData(
        backgroundColor: text,
        contentTextStyle: TextStyle(color: bg, fontSize: fsSub),
        behavior: SnackBarBehavior.floating,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(rChip)),
      ),
      listTileTheme: ListTileThemeData(
        iconColor: sub,
        textColor: text,
        dense: true,
      ),
    );
  }
}
