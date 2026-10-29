using System.Collections.Generic;

namespace Moonwell.Testing
{
#if DEBUG
    /// <summary>Cases for Moonwell's clock in a development build: halting it, stepping it, and running it faster or slower than game time, with the timers it runs.</summary>
    public static class ClockCases
    {
        /// <summary>A timer made while Moonwell's clock is halted, whose scope ends after the game has gone on 3 s, is not reported as never run: the game's own clock moved, though Moonwell's did not.</summary>
        public static void HaltedTimerEndedLaterIsQuiet(FakeWorld world)
        {
            HaltNow(world);
            var scope = new TestScope("scope", new List<string>());
            scope.After(1.0f, () => { });
            world.Advance(3.0f);
            scope.End();
            Expect.Equal(0, ScopeCases.NeverRanReports(world), "the reports: " + string.Join(" | ", world.Logged));
        }

        /// <summary>A timer made while halted, whose scope ends after a step, is not reported: Moonwell's clock moved, though the game's did not.</summary>
        public static void SteppedTimerEndedLaterIsQuiet(FakeWorld world)
        {
            HaltNow(world);
            var scope = new TestScope("scope", new List<string>());
            scope.After(1.0f, () => { });
            Step(1.0f / 32.0f);
            scope.End();
            Expect.Equal(0, ScopeCases.NeverRanReports(world), "the reports: " + string.Join(" | ", world.Logged));
        }

        /// <summary>A timer made while halted, whose scope ends in that same instant, is still reported.</summary>
        public static void HaltedTimerEndedAsMadeIsReported(FakeWorld world)
        {
            HaltNow(world);
            var scope = new TestScope("scope", new List<string>());
            scope.After(1.0f, () => { });
            scope.End();
            Expect.Equal(1, ScopeCases.NeverRanReports(world), "the reports: " + string.Join(" | ", world.Logged));
        }

        /// <summary>Halts the clock at the fake's game time.</summary>
        private static void HaltNow(FakeWorld world)
        {
            Runtime.Clock.Halt(world.Time);
        }

        /// <summary>Resumes the clock at the fake's game time, and re-arms the wake, as the caller of Resume does.</summary>
        private static void ResumeNow(FakeWorld world)
        {
            Runtime.Clock.Resume(world.Time);
            Runtime.Scheduler.Rearm();
        }

        /// <summary>Sets the clock's speed at the fake's game time, and re-arms the wake, as the caller of SetSpeed does.</summary>
        private static void ChangeSpeed(FakeWorld world, float speed)
        {
            Runtime.Clock.SetSpeed(speed, world.Time);
            Runtime.Scheduler.Rearm();
        }

        /// <summary>One step of the halted clock: Moonwell's time moves on by the seconds, and what is due runs.</summary>
        private static void Step(float seconds)
        {
            Runtime.Clock.Advance(seconds);
            Runtime.Wake();
        }

        /// <summary>A 0.5 s timer runs 2 times in 1 s; halted, 0 times in the next 3 s; resumed, 2 times in the next 1 s.</summary>
        public static void Halt(FakeWorld world)
        {
            var ran = 0;
            Game.Instance.Every(0.5f, () => { ran++; });
            world.Advance(1.0f);
            Expect.Equal(2, ran, "runs in the first second");
            HaltNow(world);
            Expect.True(Runtime.Clock.IsHalted, "the clock is halted");
            world.Advance(3.0f);
            Expect.Equal(2, ran, "runs while halted for 3 seconds");
            Expect.Near(1.0f, Runtime.World.Now(), "Moonwell's time while halted");
            ResumeNow(world);
            Expect.True(!Runtime.Clock.IsHalted, "the clock runs again");
            world.Advance(1.0f);
            Expect.Equal(4, ran, "runs in the second after the resume");
            Expect.Near(2.0f, Runtime.World.Now(), "Moonwell's time after the resume");
        }

        /// <summary>Halted, 16 steps of 1/32 s, each with a wake, run the 0.5 s timer once, at Moonwell's 0.5.</summary>
        public static void StepAdvances(FakeWorld world)
        {
            var ran = 0;
            var at = -1.0f;
            Game.Instance.Every(0.5f, () =>
            {
                ran++;
                at = Runtime.World.Now();
            });
            HaltNow(world);
            for (var i = 0; i < 15; i++)
            {
                Step(1.0f / 32.0f);
            }

            Expect.Equal(0, ran, "runs after 15 steps");
            Step(1.0f / 32.0f);
            Expect.Equal(1, ran, "runs after 16 steps");
            Expect.Near(0.5f, at, "Moonwell's time it ran at");
            Expect.True(Runtime.Clock.IsHalted, "the clock is still halted after the steps");
            world.Advance(2.0f);
            Expect.Equal(1, ran, "runs while halted for 2 seconds of game time after the steps");
        }

        /// <summary>At speed 4 the 0.5 s timer runs 8 times in 1 s of game time; at speed 0.25, once in 2 s.</summary>
        public static void Speed(FakeWorld world)
        {
            var ran = 0;
            Game.Instance.Every(0.5f, () => { ran++; });
            ChangeSpeed(world, 4.0f);
            Expect.Near(4.0f, Runtime.Clock.Speed, "the speed");
            world.Advance(1.0f);
            Expect.Equal(8, ran, "runs in 1 second at speed 4");
            Expect.Near(4.0f, Runtime.World.Now(), "Moonwell's time after 1 second at speed 4");
            ChangeSpeed(world, 0.25f);
            world.Advance(1.9f);
            Expect.Equal(8, ran, "runs in 1.9 seconds at speed 0.25");
            world.Advance(0.1f);
            Expect.Equal(9, ran, "runs in 2 seconds at speed 0.25");
            Expect.Near(4.5f, Runtime.World.Now(), "Moonwell's time after 2 seconds at speed 0.25");
        }

        /// <summary>Moonwell's time read just before and just after a change of speed and a resume is the same: it never jumps.</summary>
        public static void NoJump(FakeWorld world)
        {
            world.Advance(1.25f);
            var before = Runtime.World.Now();
            ChangeSpeed(world, 4.0f);
            Expect.Near(before, Runtime.World.Now(), "Moonwell's time across the change to speed 4");
            world.Advance(0.5f);
            Expect.Near(3.25f, Runtime.World.Now(), "Moonwell's time after 0.5 seconds at speed 4");
            HaltNow(world);
            world.Advance(2.0f);
            before = Runtime.World.Now();
            ResumeNow(world);
            Expect.Near(before, Runtime.World.Now(), "Moonwell's time across the resume");
            Expect.Near(3.25f, Runtime.World.Now(), "Moonwell's time after the halt");
            world.Advance(0.25f);
            before = Runtime.World.Now();
            ChangeSpeed(world, 0.25f);
            Expect.Near(before, Runtime.World.Now(), "Moonwell's time across the change to speed 0.25");
            Expect.Near(4.25f, Runtime.World.Now(), "Moonwell's time after 0.25 seconds more at speed 4");
        }

        /// <summary>
        /// A one-shot timer of 1 s started while halted at Moonwell's 2.0 runs once a step reaches 3.0, and not before; one of
        /// 0 s runs at the first step, not at the game wake that was asked for before the halt.
        /// </summary>
        public static void TimerWhileHalted(FakeWorld world)
        {
            world.Advance(2.0f);
            HaltNow(world);
            var ran = 0;
            var atOnce = 0;
            Game.Instance.After(1.0f, () => { ran++; });
            Game.Instance.After(0.0f, () => { atOnce++; });
            world.Advance(5.0f);
            Expect.Equal(0, ran, "runs while halted for 5 seconds of game time");
            Expect.Equal(0, atOnce, "runs of the 0 s timer while halted");
            Step(0.0f);
            Expect.Equal(1, atOnce, "runs of the 0 s timer at a step");
            for (var i = 0; i < 3; i++)
            {
                Step(0.25f);
            }

            Expect.Equal(0, ran, "runs at Moonwell's 2.75");
            Step(0.25f);
            Expect.Equal(1, ran, "runs at Moonwell's 3.0");
        }

        /// <summary>Of two timers due at 1.0, the first halts the clock: the second runs only at the next step, once.</summary>
        public static void HaltInsideTimer(FakeWorld world)
        {
            var log = new List<string>();
            Game.Instance.After(1.0f, () =>
            {
                log.Add("first");
                Runtime.Clock.Halt(world.Time);
            });
            Game.Instance.After(1.0f, () => { log.Add("second"); });
            world.Advance(1.0f);
            Expect.Equal(1, log.Count, "runs when the first halts");
            Expect.Equal("first", log[0], "the run before the halt");
            world.Advance(3.0f);
            Expect.Equal(1, log.Count, "runs while halted for 3 seconds of game time");
            Step(1.0f / 32.0f);
            Expect.Equal(2, log.Count, "runs after the next step");
            Expect.Equal("second", log[1], "the run at the step");
            Step(1.0f / 32.0f);
            world.Advance(1.0f);
            Expect.Equal(2, log.Count, "runs after another step and a second of game time");
        }

        /// <summary>
        /// At speed 8 with the game waking only every 0.1 s (0.8 s of Moonwell's time), every due timer still runs, in due order:
        /// the fake's time is moved and Runtime.Wake called by hand, as a late game timer would.
        /// </summary>
        public static void Catchup(FakeWorld world)
        {
            var dues = new List<float>();
            var halves = 0;
            var thirds = 0;
            var once = 0;
            Game.Instance.Every(0.5f, () =>
            {
                halves++;
                dues.Add(0.5f * halves);
            });
            Game.Instance.Every(0.3f, () =>
            {
                thirds++;
                dues.Add(0.3f * thirds);
            });
            Game.Instance.After(2.45f, () =>
            {
                once++;
                dues.Add(2.45f);
            });
            ChangeSpeed(world, 8.0f);
            for (var i = 1; i <= 10; i++)
            {
                world.Time = 0.1f * i;
                Runtime.Wake();
            }

            Expect.Near(8.0f, Runtime.World.Now(), "Moonwell's time after 1 second of game time at speed 8");
            Expect.Equal(16, halves, "the 0.5 s timer's runs");
            Expect.Equal(26, thirds, "the 0.3 s timer's runs");
            Expect.Equal(1, once, "the one-shot timer's runs");
            Expect.Equal(43, dues.Count, "the runs in all");
            for (var i = 1; i < dues.Count; i++)
            {
                Expect.True(dues[i - 1] <= dues[i] + 0.001f, "run " + i + " is not due before the run before it");
            }
        }
    }
#endif
}
