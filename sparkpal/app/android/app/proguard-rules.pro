# R8 rules for SparkPal.
# Keep this file ASCII-only: R8's parser rejects non-ASCII comments.

# Flutter engine and embedding
-keep class io.flutter.** { *; }
-dontwarn io.flutter.embedding.**

# Plugins used by this app
-keep class io.flutter.plugins.** { *; }
-keep class io.flutter.plugins.sharedpreferences.** { *; }

# Keep line numbers so crash stacks match the symbol file
-keepattributes SourceFile,LineNumberTable
-renamesourcefileattribute SourceFile

# Annotations / signatures used by reflection and JSON
-keepattributes *Annotation*
-keepattributes Signature
-keepclassmembers class * {
    @com.google.gson.annotations.SerializedName <fields>;
}
