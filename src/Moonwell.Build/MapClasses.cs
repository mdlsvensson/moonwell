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

            var frame = FirstFrameInProject(new StackTrace(inner, true), project);
            if (inner.GetType().FullName == FieldRuleExceptionName)
            {
                // A value its field does not allow: the message names the class and the property. A formula of levels is
                // checked after the constructor, so its line is where the formula was put.
                var putAt = (StackTrace?)inner.GetType().GetProperty("PutAt")!.GetValue(inner);
                frame ??= putAt == null ? null : FirstFrameInProject(putAt, project);
                return new BuildException((frame == null ? string.Empty : LineOf(frame, project) + ": ") + RuleMessage(inner, frame));
            }

            var place = frame == null ? null : LineOf(frame, project);
            var prefix = place == null ? string.Empty : place + ": ";
            if (inner is BadImageFormatException || IsLiveMemberWithoutGame(inner))
            {
                return new BuildException($"{prefix}{name} calls the game while it is declared; game work starts from a reaction or the game's start");
            }

            return new BuildException($"{prefix}{name}'s constructor threw: {inner.Message}");
        }

        // A unit's, hero's or item's live value or action (Mana = 300, which MW0012 rejects) reaches for the game, which is not
        // there while the class is declared: a NullReferenceException thrown in Moonwell's Unit, Hero or Item itself.
        private static bool IsLiveMemberWithoutGame(Exception inner)
        {
            if (inner is not NullReferenceException)
            {
                return false;
            }

            var thrownIn = new StackTrace(inner, false).GetFrame(0)?.GetMethod()?.DeclaringType?.FullName;
            return thrownIn == "Moonwell.Unit" || thrownIn == "Moonwell.Hero" || thrownIn == "Moonwell.Item";
        }

        internal static string? FirstLineInProject(StackTrace trace, MapProject project)
        {
            var frame = FirstFrameInProject(trace, project);
            return frame == null ? null : LineOf(frame, project);
        }

        private static StackFrame? FirstFrameInProject(StackTrace trace, MapProject project)
        {
            var root = project.Folder + Path.DirectorySeparatorChar;
            foreach (var frame in trace.GetFrames())
            {
                var file = frame.GetFileName();
                if (file != null && file.StartsWith(root, StringComparison.OrdinalIgnoreCase))
                {
                    return frame;
                }
            }

            return null;
        }

        // The error format names the file with '/' on every system: Heroes/Arthas.cs:12.
        private static string LineOf(StackFrame frame, MapProject project)
        {
            return $"{Path.GetRelativePath(project.Folder, frame.GetFileName()!).Replace('\\', '/')}:{frame.GetFileLineNumber()}";
        }

        // A rule's message names the class whose object was made (Drillmaster.MaxLife); when its line is in a declared class's
        // own code, such as the constructor of the base it inherits (Sentry), it names that class instead, as the line does.
        private static string RuleMessage(Exception rule, StackFrame? frame)
        {
            var property = (string)rule.GetType().GetProperty("Property")!.GetValue(rule)!;
            var type = frame?.GetMethod()?.DeclaringType;
            while (type is { IsNested: true } && type.Name.StartsWith('<'))
            {
                // A class the compiler made for a lambda or an iterator, inside the class whose code it is.
                type = type.DeclaringType;
            }

            if (type == null || Declarations.KindBaseOf(type) == null || !rule.Message.StartsWith(property, StringComparison.Ordinal))
            {
                return rule.Message;
            }

            var name = type.Name;
            for (var outer = type.DeclaringType; outer != null; outer = outer.DeclaringType)
            {
                name = outer.Name + "." + name;
            }

            return name + property.Substring(property.LastIndexOf('.')) + rule.Message.Substring(property.Length);
        }
    }
}
