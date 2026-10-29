using System;
using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>Cases that check timers: when they run, how often, their order, their owner, and their errors.</summary>
    public static class TimerCases
    {
        /// <summary>A one-shot timer runs once, when its delay has passed, and then it has ended.</summary>
        public static void After(FakeWorld world)
        {
            var ran = 0;
            var timer = Game.Instance.After(2f, () => { ran++; });
            world.Advance(1.9f);
            Expect.Equal(0, ran, "runs after 1.9 seconds");
            world.Advance(0.2f);
            Expect.Equal(1, ran, "runs after 2.1 seconds");
            Expect.True(timer.HasEnded, "the timer has ended after it ran");
        }

        /// <summary>A repeating timer runs at each interval that has passed.</summary>
        public static void Every(FakeWorld world)
        {
            var ran = 0;
            Game.Instance.Every(1f, () => { ran++; });
            world.Advance(3.05f);
            Expect.Equal(3, ran, "runs after 3.05 seconds");
        }

        /// <summary>A repeating timer keeps its schedule: 0.1 seconds for 10 seconds runs exactly 100 times.</summary>
        public static void NoDrift(FakeWorld world)
        {
            var ran = 0;
            Game.Instance.Every(0.1f, () => { ran++; });
            world.Advance(10.0f);
            Expect.Equal(100, ran, "runs after 10 seconds at 0.1 seconds each");
        }

        /// <summary>
        /// A repeating timer keeps its schedule when every wake comes late, as the game's do at a frame: woken 0.01 seconds after
        /// each due time, 0.1 seconds for 10 seconds still runs 100 times, the 100th at 10.01 seconds and not 100 late wakes behind.
        /// </summary>
        public static void NoDriftWokenLate(FakeWorld world)
        {
            world.WakeLate = 0.01f;
            var ran = 0;
            var hundredth = 0.0f;
            Game.Instance.Every(0.1f, () =>
            {
                ran++;
                if (ran == 100)
                {
                    hundredth = world.Now();
                }
            });
            world.Advance(10.05f);
            Expect.Equal(100, ran, "runs after 10.05 seconds at 0.1 seconds each, each woken 0.01 seconds late");
            Expect.Near(10.01f, hundredth, "the time of the 100th run");
        }

        /// <summary>Timers due at the same time run in the order they were made.</summary>
        public static void SameTimeInCreationOrder(FakeWorld world)
        {
            var log = new List<string>();
            Game.Instance.After(1f, () => { log.Add("a"); });
            Game.Instance.After(1f, () => { log.Add("b"); });
            world.Advance(1f);
            Expect.Equal(2, log.Count, "runs after 1 second");
            Expect.Equal("a", log[0], "the first run");
            Expect.Equal("b", log[1], "the second run");
        }

        /// <summary>
        /// Many timers at once run in due order, those due at the same time in the order they were made, each at its due time: 16
        /// one-shot timers made at shuffled times, three pairs of them equal, share the scheduler's heap with the runtime's sweep.
        /// </summary>
        public static void ManyInDueOrder(FakeWorld world)
        {
            var dues = new float[] { 9f, 3f, 7f, 1f, 8f, 2f, 6f, 4f, 5f, 10f, 12f, 11f, 3f, 7f, 1.5f, 6f };
            var order = new List<int>();
            var times = new List<float>();
            for (var i = 0; i < dues.Length; i++)
            {
                var index = i;
                Game.Instance.After(dues[index], () =>
                {
                    order.Add(index);
                    times.Add(world.Now());
                });
            }

            world.Advance(13f);
            var expected = new int[] { 3, 14, 5, 1, 12, 7, 8, 6, 15, 2, 13, 4, 0, 9, 11, 10 };
            Expect.Equal(expected.Length, order.Count, "the timers that ran");
            for (var i = 0; i < expected.Length; i++)
            {
                Expect.Equal(expected[i], order[i], "the timer of run " + i);
                Expect.Near(dues[expected[i]], times[i], "the time of run " + i);
            }
        }

        /// <summary>A timer that has ended never runs.</summary>
        public static void EndCancels(FakeWorld world)
        {
            var ran = 0;
            var timer = Game.Instance.After(1f, () => { ran++; });
            timer.End();
            Expect.True(timer.HasEnded, "the timer has ended");
            world.Advance(2f);
            Expect.Equal(0, ran, "runs after it was ended");
        }

        /// <summary>A timer made inside an owner's Run is adopted by that owner, and ends with it.</summary>
        public static void OwnerAdopts(FakeWorld world)
        {
            var log = new List<string>();
            var owner = new TestScope("owner", log);
            Timer? made = null;
            made = owner.After(2f, () => { });
            var timer = made!;
            Expect.Same(owner, timer.Parent, "the timer's owner");
            owner.End();
            Expect.True(timer.HasEnded, "the timer has ended with its owner");
        }

        /// <summary>A timer that throws is reported, and a timer due at the same time still runs.</summary>
        public static void ErrorIsolated(FakeWorld world)
        {
            var log = new List<string>();
            var owner = new TestScope("owner", log);
            var second = 0;
            owner.After(1f, () => { throw new InvalidOperationException("boom"); });
            owner.After(1f, () => { second++; });
            world.Advance(1f);
            Expect.Equal(1, second, "runs of the timer after the one that threw");
            var reported = false;
            foreach (var line in world.Logged)
            {
                if (line.Contains("TestScope.Timer failed"))
                {
                    reported = true;
                }
            }

            Expect.True(reported, "a logged line names the timer that failed");
        }

        /// <summary>Remaining is the time until the timer is due.</summary>
        public static void Remaining(FakeWorld world)
        {
            var timer = Game.Instance.After(5f, () => { });
            world.Advance(2f);
            Expect.Near(3f, timer.Remaining, "remaining after 2 of 5 seconds");
        }
    }
}
