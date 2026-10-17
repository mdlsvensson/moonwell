using System;

namespace Moonwell.Testing
{
    /// <summary>Cases that check the error lines, the error throttle, the names, and the world's clock.</summary>
    public static class ErrorCases
    {
        /// <summary>The log line of an error is the head and the message, and the count shows when it repeats.</summary>
        public static void LogLine(FakeWorld world)
        {
            Expect.Equal("Moonwell: Rage.OnDamaged failed: boom", Errors.LogLine("Rage.OnDamaged", "boom", 1), "log line at 1");
            Expect.Equal("Moonwell: Rage.OnDamaged failed 10 times: boom", Errors.LogLine("Rage.OnDamaged", "boom", 10), "log line at 10");
        }

        /// <summary>The screen line is coloured, and a release build shows the head only.</summary>
        public static void ScreenLine(FakeWorld world)
        {
            Expect.Equal("|cffff4040Moonwell: Rage.OnDamaged failed:|r boom", Errors.ScreenLine("Rage.OnDamaged", "boom", 1, false), "development screen line");
            Expect.Equal("|cffff4040Moonwell: Rage.OnDamaged failed|r", Errors.ScreenLine("Rage.OnDamaged", "boom", 1, true), "release screen line");
        }

        /// <summary>An error shows at its first time, and then at every power of ten.</summary>
        public static void ShowsAt(FakeWorld world)
        {
            var shown = new int[] { 1, 10, 100, 1000 };
            for (var i = 0; i < shown.Length; i++)
            {
                Expect.True(Errors.ShowsAt(shown[i]), "shows at " + shown[i]);
            }

            var hidden = new int[] { 2, 9, 11, 99, 101 };
            for (var i = 0; i < hidden.Length; i++)
            {
                Expect.True(!Errors.ShowsAt(hidden[i]), "hides at " + hidden[i]);
            }
        }

        /// <summary>The same error reported twelve times is logged at its first and tenth time only.</summary>
        public static void ReportThrottles(FakeWorld world)
        {
            for (var i = 0; i < 12; i++)
            {
                Errors.Report("Rage.OnDamaged", new InvalidOperationException("boom"));
            }

            Expect.Equal(2, world.Logged.Count, "log lines after twelve identical errors");
            Expect.Equal("Moonwell: Rage.OnDamaged failed 10 times: boom", world.Logged[1], "the second log line");
        }

        /// <summary>Only a hundred distinct errors are remembered; a later one is logged every time it happens.</summary>
        public static void ReportManyKeys(FakeWorld world)
        {
            for (var i = 0; i < 101; i++)
            {
                Errors.Report("Rage.OnDamaged", new InvalidOperationException("message " + i));
            }

            Errors.Report("Rage.OnDamaged", new InvalidOperationException("message 100"));
            Expect.Equal(102, world.Logged.Count, "log lines after 101 distinct errors and the 101st again");
        }

        /// <summary>The name of an object is its class name, and two objects of one class share the class.</summary>
        public static void NamesOf(FakeWorld world)
        {
            Expect.Equal("ExpectationFailed", Names.NameOf(new ExpectationFailed("x")), "the name of an ExpectationFailed");
            var first = Names.ClassOf(new ExpectationFailed("a"));
            var second = Names.ClassOf(new ExpectationFailed("b"));
            Expect.Same(first, second, "the class of two ExpectationFailed objects");
            Expect.True(first != Names.ClassOf(new MoonwellException("c")), "the class of a MoonwellException is another class");
        }

        /// <summary>A wake request fires when time reaches it, and time moves on to the target after it.</summary>
        public static void WorldAdvance(FakeWorld world)
        {
            world.WakeIn(2f);
            var wakeAt = world.WakeAt ?? -1f;
            Expect.Near(2f, wakeAt, "WakeAt after WakeIn(2), -1 when none is pending");
            world.Advance(1f);
            Expect.Near(1f, world.Time, "Time after Advance(1)");
            world.Advance(1.5f);
            Expect.Near(2.5f, world.Time, "Time after Advance(1.5)");
        }
    }
}
