using System.Diagnostics;
using System.Reflection;

namespace Moonwell.Build
{
    // What the readers of a built map assembly share: how a class is named, and how a failing constructor is reported.
    internal static class MapClasses
    {
        // Moonwell.Objects is loaded in the map's load context, so its types are known by name.
        private const string FieldRuleExceptionName = "Moonwell.Objects.FieldRuleException";

        public static string Describe(Type type)
        {
            return type.FullName!.Replace('+', '.');
        }

        public static BuildException ConstructionError(Exception inner, string name, MapProject project)
        {
            // A static field or a static constructor that throws surfaces as the type initializer's failure.
            while (inner is TypeInitializationException { InnerException: not null })
            {
                inner = inner.InnerException!;
            }

            var place = FirstLineInProject(new StackTrace(inner, true), project);
            if (inner.GetType().FullName == FieldRuleExceptionName)
            {
                // A value its field does not allow: the message names the class and the property. A formula of levels is
                // checked after the constructor, so its line is where the formula was put.
                var putAt = (StackTrace?)inner.GetType().GetProperty("PutAt")!.GetValue(inner);
                place ??= putAt == null ? null : FirstLineInProject(putAt, project);
                return new BuildException((place == null ? string.Empty : place + ": ") + inner.Message);
            }

            var prefix = place == null ? string.Empty : place + ": ";
            if (inner is BadImageFormatException)
            {
                return new BuildException($"{prefix}{name} calls the game while it is declared; game work starts from a reaction or the game's start");
            }

            return new BuildException($"{prefix}{name}'s constructor threw: {inner.Message}");
        }

        private static string? FirstLineInProject(StackTrace trace, MapProject project)
        {
            var root = project.Folder + Path.DirectorySeparatorChar;
            foreach (var frame in trace.GetFrames())
            {
                var file = frame.GetFileName();
                if (file != null && file.StartsWith(root, StringComparison.OrdinalIgnoreCase))
                {
                    // The error format names the file with '/' on every system: Heroes/Arthas.cs:12.
                    return $"{Path.GetRelativePath(project.Folder, file).Replace('\\', '/')}:{frame.GetFileLineNumber()}";
                }
            }

            return null;
        }
    }
}
