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

        /// <summary>How many logged lines hold the text.</summary>
        private static int LoggedWith(FakeWorld world, string text)
        {
            var found = 0;
            foreach (var line in world.Logged)
            {
                if (line.Contains(text))
                {
                    found++;
                }
            }

            return found;
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

        /// <summary>
        /// Timers made out of due order within a few milliseconds run in due order: a at 0.0016 s, b at 0.0008 s, two fillers,
        /// then c at 0 s. The wake at 0 s runs c, and b, which is within the scheduler's 0.001 s; a runs at its own time.
        /// </summary>
        public static void NearTimesOrder(FakeWorld world)
        {
            var log = new List<string>();
            Timer.After(0.0016f, () => { log.Add("a"); });
            Timer.After(0.0008f, () => { log.Add("b"); });
            Timer.After(0.3f, () => { });
            Timer.After(0.3f, () => { });
            Timer.After(0.0f, () => { log.Add("c"); });
            world.Advance(0.0005f);
            var early = log.Count;
            world.Advance(0.01f);
            Expect.Equal("2 c b a", early + " " + string.Join(" ", log), "runs by 0.0005 s, then the order of the runs");
        }

        /// <summary>
        /// Twenty one-shot timers due 0.0004 s apart, made latest first, never run more than the scheduler's 0.001 s out of due
        /// order.
        /// </summary>
        public static void NearTimesMany(FakeWorld world)
        {
            var dues = new List<float>();
            for (var i = 19; i >= 0; i--)
            {
                var due = 0.0004f * i;
                Timer.After(due, () => { dues.Add(due); });
            }

            world.Advance(0.1f);
            Expect.Equal(20, dues.Count, "the timers that ran");
            for (var i = 1; i < dues.Count; i++)
            {
                Expect.True(dues[i] >= dues[i - 1] - 0.001f, "run " + i + " (due " + dues[i] + ") is not more than 0.001 s before the run before it (due " + dues[i - 1] + ")");
            }
        }

        /// <summary>After(0) made inside a timer runs in the same instant, as the game's own 0 s timer does: a chain of five links made at 1.1 s all run at 1.1 s.</summary>
        public static void AfterZeroSameInstant(FakeWorld world)
        {
            var times = new List<float>();
            Action? link = null;
            link = () =>
            {
                times.Add(Runtime.World.Now());
                if (times.Count < 5)
                {
                    Timer.After(0.0f, link!);
                }
            };
            Timer.After(1.1f, link);
            world.Advance(1.1f);
            Expect.Equal(5, times.Count, "links run by 1.1 s");
            for (var i = 0; i < times.Count; i++)
            {
                Expect.Near(1.1f, times[i], "Moonwell's time of link " + i);
            }
        }

        /// <summary>
        /// A timer that restarts itself with After(0) is a runaway: 10000 runs at 1.1 s, then it is reported once, naming its
        /// owner, and the rest wait 1/32 s; 10000 more at 1.13125 s; it stops when its owner ends. The case's chain stops by itself
        /// at 30000 links.
        /// </summary>
        public static void AfterZeroRunaway(FakeWorld world)
        {
            var owner = new TestOwner("owner", new List<string>());
            var runs = 0;
            Action? link = null;
            link = () =>
            {
                runs++;
                if (runs < 30000)
                {
                    Timer.After(0.0f, link!);
                }
            };
            owner.Run(() => { Timer.After(1.1f, link); });
            world.Advance(1.1f);
            var first = runs;
            world.Advance(0.03f);
            var waited = runs;
            world.Advance(0.002f);
            var second = runs;
            owner.End();
            world.Advance(1.0f);
            var reported = LoggedWith(world, "TestOwner.Timer failed: more than 10000 timer runs in one instant");
            Expect.Equal("10000 10000 20000 20000 1", first + " " + waited + " " + second + " " + runs + " " + reported, "runs at 1.1 s, at 1.13 s, at 1.132 s and after its owner ended, and the lines reported");
        }

        /// <summary>
        /// More than 10000 timers due in one instant is a runaway too: of 12000 one-shot timers due at 1.1 s, 10000 run then, one
        /// line is reported, and the other 2000 run 1/32 s later.
        /// </summary>
        public static void RunawayManyTimers(FakeWorld world)
        {
            var runs = 0;
            for (var i = 0; i < 12000; i++)
            {
                Timer.After(1.1f, () => { runs++; });
            }

            world.Advance(1.1f);
            var first = runs;
            world.Advance(0.032f);
            var reported = LoggedWith(world, "Game.Timer failed: more than 10000 timer runs in one instant");
            Expect.Equal("10000 12000 1", first + " " + runs + " " + reported, "runs at 1.1 s and at 1.132 s, and the lines reported");
        }

        /// <summary>A repeating timer catching up after a slow frame is no runaway: Every(0.001) runs 5000 times in the wake after a frame of 5 s, and nothing is reported.</summary>
        public static void CatchUpNotRunaway(FakeWorld world)
        {
            var runs = 0;
            Timer.Every(0.001f, () => { runs++; });
            world.Time = 5.0f;
            Runtime.Wake();
            Expect.Equal("5000 0", runs + " " + world.Logged.Count, "runs in the wake after a frame of 5 s, and the lines reported");
        }

        /// <summary>
        /// Every with 0, a negative period or NaN is reported and refused in every build: each gives an ended timer that never
        /// runs, and the log names Timer.Every. The case ends each after 10000 runs, which it reaches in one instant when it runs.
        /// </summary>
        public static void EveryZeroRefused(FakeWorld world)
        {
            var runs = 0;
            var timers = new List<Timer>();
            var periods = new float[] { 0.0f, -1.0f, float.NaN };
            for (var i = 0; i < periods.Length; i++)
            {
                Timer? timer = null;
                timer = Timer.Every(periods[i], () =>
                {
                    runs++;
                    if (runs % 10000 == 0)
                    {
                        timer!.End();
                    }
                });
                timers.Add(timer);
            }

            world.Advance(1.0f);
            var ended = 0;
            foreach (var timer in timers)
            {
                if (timer.HasEnded)
                {
                    ended++;
                }
            }

            var reported = LoggedWith(world, "Timer.Every failed: seconds must be more than 0");
            Expect.Equal("0 3 3", runs + " " + ended + " " + reported, "runs of Every(0), Every(-1) and Every(NaN) in 1 s, the timers ended, and the lines reported");
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
