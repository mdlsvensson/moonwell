using System.Diagnostics;
using System.Reflection;

namespace Moonwell.Build
{
    // What the readers of a built map assembly share: how a class is named, and how a failing constructor is reported.
    internal static class MapClasses
    {
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

            var place = FirstLineInProject(inner, project);
            var prefix = place == null ? string.Empty : place + ": ";
            if (inner is BadImageFormatException)
            {
                return new BuildException($"{prefix}{name} calls the game while it is declared; game work starts from a reaction or the game's start");
            }

            return new BuildException($"{prefix}{name}'s constructor threw: {inner.Message}");
        }

        private static string? FirstLineInProject(Exception exception, MapProject project)
        {
            var root = project.Folder + Path.DirectorySeparatorChar;
            foreach (var frame in new StackTrace(exception, true).GetFrames())
            {
                var file = frame.GetFileName();
                if (file != null && file.StartsWith(root, StringComparison.OrdinalIgnoreCase))
                {
                    return $"{Path.GetRelativePath(project.Folder, file)}:{frame.GetFileLineNumber()}";
                }
            }

            return null;
        }
    }
}
