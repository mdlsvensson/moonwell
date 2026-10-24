using System;
using System.Collections.Generic;

namespace Moonwell
{
    /// <summary>Reports an error from a hook to the screen and the log, and throttles an error that repeats.</summary>
    internal static class Errors
    {
        /// <summary>How many distinct errors are remembered; a later one is reported every time.</summary>
        private const int MaxKeys = 100;

        /// <summary>How many times each remembered error has happened, keyed by its label and message.</summary>
        private static readonly Dictionary<string, int> counts = new Dictionary<string, int>();

        /// <summary>Reports an error from a hook, shown at its first time and then at every power of ten.</summary>
        internal static void Report(string label, Exception error)
        {
#if DEBUG
            const bool release = false;
#else
            const bool release = true;
#endif
            var where = Where(error);
            var key = label + "\n" + error.Message + "\n" + First(where);
            var times = 1;
            if (counts.TryGetValue(key, out var seen))
            {
                times = seen + 1;
                counts[key] = times;
            }
            else if (counts.Count < MaxKeys)
            {
                counts[key] = 1;
            }

            if (ShowsAt(times))
            {
                Runtime.World.Print(ScreenLine(label, error.Message, times, release, where));
                Runtime.World.Log(LogLine(label, error.Message, times, where));
            }
        }

        /// <summary>Forgets every error count; Runtime.Reset calls it.</summary>
        internal static void Reset()
        {
            counts.Clear();
        }

        /// <summary>
        /// The C# locations an error was raised at, innermost first, one a line; empty when none is known. In Lua MoonwellWhere
        /// (moonwell.lua) reads what the exception's traceback recorded; .NET has none.
        /// </summary>
        /// @CSharpLua.Template = "MoonwellWhere({0})"
        internal static string Where(Exception error)
        {
            return string.Empty;
        }

        /// <summary>The line appended to the log for an error, with every location an error's Where gave.</summary>
        internal static string LogLine(string label, string message, int times, string where)
        {
            var line = Head(label, times) + ": " + message;
            if (where.Length == 0)
            {
                return line;
            }

            var newline = where.IndexOf('\n');
            if (newline < 0)
            {
                return line + " (" + where + ")";
            }

            return line + " (" + where.Substring(0, newline) + ", from " + where.Substring(newline + 1).Replace("\n", ", ") + ")";
        }

        /// <summary>The line shown on the screen for an error, with its innermost location; a release build leaves the message out.</summary>
        internal static string ScreenLine(string label, string message, int times, bool release, string where)
        {
            var first = First(where);
            var location = first.Length == 0 ? string.Empty : " (" + first + ")";
            if (release)
            {
                return "|cffff4040" + Head(label, times) + "|r" + location;
            }

            return "|cffff4040" + Head(label, times) + ":|r " + message + location;
        }

        /// <summary>Whether an error that has happened this many times is shown: 1, 10, 100, 1000 and every power of ten.</summary>
        internal static bool ShowsAt(int times)
        {
            if (times < 1)
            {
                return false;
            }

            var rest = times;
            while (rest % 10 == 0)
            {
                rest /= 10;
            }

            return rest == 1;
        }

        /// <summary>The innermost of the locations, or empty.</summary>
        private static string First(string where)
        {
            var newline = where.IndexOf('\n');
            return newline < 0 ? where : where.Substring(0, newline);
        }

        /// <summary>The start of an error's line, with the count once it has repeated.</summary>
        private static string Head(string label, int times)
        {
            if (times == 1)
            {
                return "Moonwell: " + label + " failed";
            }

            return "Moonwell: " + label + " failed " + times + " times";
        }
    }
}
