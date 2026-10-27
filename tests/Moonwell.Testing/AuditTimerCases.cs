using System;
using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>A buff of 2 s that ticks every 0.2 s and counts its ticks.</summary>
    public sealed class DriftTicker : Buff
    {
        /// <summary>How many times the buff has ticked.</summary>
        public int Ticks;

        /// <summary>Makes the buff with its duration and tick.</summary>
        public DriftTicker()
        {
            Duration = 2.0f;
            Tick = 0.2f;
        }

        /// <summary>Counts a tick.</summary>
        public override void OnTick()
        {
            Ticks++;
        }
    }

    /// <summary>
    /// Cases from the full audit's timer findings. .NET's float is single precision like the game's, so the cases late in a game
    /// see the game's rounding; Lua on the test host has 64-bit numbers, where they pass either way.
    /// </summary>
    public static class AuditTimerCases
    {
        /// <summary>Starts the runtime again on the world at a game time, as if the game had run that long.</summary>
        private static void StartAt(FakeWorld world, float time)
        {
            world.Time = time;
            Runtime.Reset(world);
            Runtime.ListenForCarriers();
        }

        /// <summary>The runs of a repeating timer over the seconds, with the runtime started again at a game time.</summary>
        private static int RunsFrom(FakeWorld world, float time, float interval, float seconds)
        {
            StartAt(world, time);
            var runs = 0;
            Timer.Every(interval, () => { runs++; });
            world.Advance(seconds);
            return runs;
        }

        /// <summary>
        /// A repeating timer keeps its rate late in a game, where a float moves in steps of about 0.0005 s (from 68 min) and
        /// 0.002 s (from 4.5 h): Every(0.01) runs 1000 times in 10 s from 0 s and from 7200 s, and Every(0.03) 1000 times in
        /// 30 s from 20000 s.
        /// </summary>
        public static void EveryLateNoDrift(FakeWorld world)
        {
            var early = 0;
            Timer.Every(0.01f, () => { early++; });
            world.Advance(10.0f);
            var late = RunsFrom(world, 7200.0f, 0.01f, 10.0f);
            var later = RunsFrom(world, 20000.0f, 0.03f, 30.0f);
            Expect.Equal("1000 1000 1000", early + " " + late + " " + later, "runs of Every(0.01) over 10 s from 0 s and from 7200 s, and of Every(0.03) over 30 s from 20000 s");
        }

        /// <summary>
        /// A repeating timer shorter than a float step still runs at its rate: Every(0.0001) at 3600 s, where a float moves in
        /// steps of 0.00024 s, runs about 10000 times in a second. The case ends it after 100000 runs, which it reaches in one
        /// instant when its due time stands still.
        /// </summary>
        public static void EveryTinyLate(FakeWorld world)
        {
            StartAt(world, 3600.0f);
            var runs = 0;
            Timer? timer = null;
            timer = Timer.Every(0.0001f, () =>
            {
                runs++;
                if (runs >= 100000)
                {
                    timer!.End();
                }
            });
            world.Advance(0.5f);
            var first = runs;
            world.Advance(1.0f);
            var second = runs - first;
            Expect.True(first >= 4990 && first <= 5020 && second >= 9990 && second <= 10010, "runs of Every(0.0001) from 3600 s in the first 0.5 s and the next 1 s: expected about 5000 and 10000, got " + first + " and " + second);
        }

        /// <summary>
        /// Every(0.001) at 33000 s, where a float moves in steps of 0.0039 s, runs about 1000 times in a second: several runs share
        /// one instant. The case ends it after 100000 runs, which it reaches in one instant when its due time stands still.
        /// </summary>
        public static void EveryMilliVeryLate(FakeWorld world)
        {
            StartAt(world, 33000.0f);
            var runs = 0;
            Timer? timer = null;
            timer = Timer.Every(0.001f, () =>
            {
                runs++;
                if (runs >= 100000)
                {
                    timer!.End();
                }
            });
            world.Advance(1.0f);
            var first = runs;
            world.Advance(1.0f);
            var second = runs - first;
            Expect.True(first >= 990 && first <= 1010 && second >= 990 && second <= 1010, "runs of Every(0.001) from 33000 s in the first and the second second: expected about 1000 each, got " + first + " and " + second);
        }

        /// <summary>A buff of 2 s ticking every 0.2 s ticks 10 times, applied at 0 s and at 5000 s: its last tick runs before its expiry.</summary>
        public static void BuffTicksLate(FakeWorld world)
        {
            var early = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            var earlyBuff = early.Apply(new DriftTicker());
            world.Advance(3.0f);
            StartAt(world, 5000.0f);
            var late = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            var lateBuff = late.Apply(new DriftTicker());
            world.Advance(3.0f);
            Expect.Equal("10 10", earlyBuff.Ticks + " " + lateBuff.Ticks, "ticks of the buff applied at 0 s and at 5000 s");
        }
#if DEBUG

        /// <summary>
        /// Moonwell's clock at speed 4 from 7200 s keeps a repeating timer's rate: Every(0.01) runs 4000 times in 10 s of game
        /// time, 40 s of Moonwell's.
        /// </summary>
        public static void NoDriftAtSpeed(FakeWorld world)
        {
            StartAt(world, 7200.0f);
            Runtime.Clock.SetSpeed(4.0f, world.Time);
            Runtime.Scheduler.Rearm();
            var runs = 0;
            Timer.Every(0.01f, () => { runs++; });
            world.Advance(10.0f);
            Expect.Equal(4000, runs, "runs of Every(0.01) over 10 s of game time at speed 4 from 7200 s");
            Expect.Near(7240.0f, Runtime.World.Now(), "Moonwell's time after 10 s of game time at speed 4");
        }
#endif
    }
}
