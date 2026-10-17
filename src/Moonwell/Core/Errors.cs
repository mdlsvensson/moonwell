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
            var key = label + "\n" + error.Message;
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
                Runtime.World.Print(ScreenLine(label, error.Message, times, release));
                Runtime.World.Log(LogLine(label, error.Message, times));
            }
        }

        /// <summary>Forgets every error count; Runtime.Reset calls it.</summary>
        internal static void Reset()
        {
            counts.Clear();
        }

        /// <summary>The line appended to the log for an error.</summary>
        internal static string LogLine(string label, string message, int times)
        {
            return Head(label, times) + ": " + message;
        }

        /// <summary>The line shown on the screen for an error; a release build shows the head only.</summary>
        internal static string ScreenLine(string label, string message, int times, bool release)
        {
            if (release)
            {
                return "|cffff4040" + Head(label, times) + "|r";
            }

            return "|cffff4040" + Head(label, times) + ":|r " + message;
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
