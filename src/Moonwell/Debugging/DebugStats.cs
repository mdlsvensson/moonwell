#if DEBUG
namespace Moonwell
{
    /// <summary>
    /// The statistics core keeps for the debug mode: the real time Moonwell's timers took over the last second. Times are
    /// os.clock's, in 1 ms steps (game facts), so they are summed over the last 32 refreshes of the panel (one second of
    /// game time while running) rather than shown a wake at a time.
    /// </summary>
    internal static class DebugStats
    {
        /// <summary>The refreshes a second, and the sums kept.</summary>
        private const int Steps = 32;

        /// <summary>The seconds the timers took in each of the last 32 refreshes, round from current.</summary>
        private static float[] spent = new float[Steps];

        /// <summary>The place in spent the next refresh fills.</summary>
        private static int current;

        /// <summary>The seconds the timers took since the last refresh.</summary>
        private static float running;

        /// <summary>Adds the real seconds one wake of Moonwell's timers took; Scheduler.Wake calls it while the mode is on.</summary>
        internal static void TimersRan(float seconds)
        {
            running += seconds;
        }

        /// <summary>Closes one refresh's sum: the oldest of the 32 is dropped. The panel's refresher calls it every 1/32 s of game time while running.</summary>
        internal static void Tick()
        {
            spent[current] = running;
            running = 0.0f;
            current = (current + 1) % Steps;
        }

        /// <summary>"Moonwell's timers 0.3 ms a second": the milliseconds the timers took over the last 32 refreshes.</summary>
        internal static string TimersLine()
        {
            var sum = 0.0f;
            for (var i = 0; i < Steps; i++)
            {
                sum += spent[i];
            }

            return "Moonwell's timers " + DebugPanel.Tenths(sum * 1000.0f) + " ms a second";
        }

        /// <summary>Forgets every sum; Runtime.Reset calls it.</summary>
        internal static void Reset()
        {
            spent = new float[Steps];
            current = 0;
            running = 0.0f;
        }
    }
}
#endif
