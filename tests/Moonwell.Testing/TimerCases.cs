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
            var timer = Timer.After(2f, () => { ran++; });
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
            Timer.Every(1f, () => { ran++; });
            world.Advance(3.05f);
            Expect.Equal(3, ran, "runs after 3.05 seconds");
        }

        /// <summary>A repeating timer keeps its schedule: 0.1 seconds for 10 seconds runs exactly 100 times.</summary>
        public static void NoDrift(FakeWorld world)
        {
            var ran = 0;
            Timer.Every(0.1f, () => { ran++; });
            world.Advance(10.0f);
            Expect.Equal(100, ran, "runs after 10 seconds at 0.1 seconds each");
        }

        /// <summary>Timers due at the same time run in the order they were made.</summary>
        public static void SameTimeInCreationOrder(FakeWorld world)
        {
            var log = new List<string>();
            Timer.After(1f, () => { log.Add("a"); });
            Timer.After(1f, () => { log.Add("b"); });
            world.Advance(1f);
            Expect.Equal(2, log.Count, "runs after 1 second");
            Expect.Equal("a", log[0], "the first run");
            Expect.Equal("b", log[1], "the second run");
        }

        /// <summary>A timer that has ended never runs.</summary>
        public static void EndCancels(FakeWorld world)
        {
            var ran = 0;
            var timer = Timer.After(1f, () => { ran++; });
            timer.End();
            Expect.True(timer.HasEnded, "the timer has ended");
            world.Advance(2f);
            Expect.Equal(0, ran, "runs after it was ended");
        }

        /// <summary>A timer made inside an owner's Run is adopted by that owner, and ends with it.</summary>
        public static void OwnerAdopts(FakeWorld world)
        {
            var log = new List<string>();
            var owner = new TestOwner("owner", log);
            Timer? made = null;
            owner.Run(() => { made = Timer.After(2f, () => { }); });
            var timer = made!;
            Expect.Same(owner, timer.Owner, "the timer's owner");
            owner.End();
            Expect.True(timer.HasEnded, "the timer has ended with its owner");
        }

        /// <summary>Inside the callback, Owner.Current is the timer's owner.</summary>
        public static void CurrentInCallback(FakeWorld world)
        {
            var log = new List<string>();
            var owner = new TestOwner("owner", log);
            Owner? seen = null;
            owner.Run(() => { Timer.After(1f, () => { seen = Owner.Current; }); });
            world.Advance(1f);
            Expect.Same(owner, seen, "Owner.Current inside the callback");
        }

        /// <summary>A timer that throws is reported, and a timer due at the same time still runs.</summary>
        public static void ErrorIsolated(FakeWorld world)
        {
            var log = new List<string>();
            var owner = new TestOwner("owner", log);
            var second = 0;
            owner.Run(() =>
            {
                Timer.After(1f, () => { throw new InvalidOperationException("boom"); });
                Timer.After(1f, () => { second++; });
            });
            world.Advance(1f);
            Expect.Equal(1, second, "runs of the timer after the one that threw");
            var reported = false;
            foreach (var line in world.Logged)
            {
                if (line.Contains("TestOwner.Timer failed"))
                {
                    reported = true;
                }
            }

            Expect.True(reported, "a logged line names the timer that failed");
        }

        /// <summary>Remaining is the time until the timer is due.</summary>
        public static void Remaining(FakeWorld world)
        {
            var timer = Timer.After(5f, () => { });
            world.Advance(2f);
            Expect.Near(3f, timer.Remaining, "remaining after 2 of 5 seconds");
        }
    }
}
