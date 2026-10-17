using Moonwell.Hosting;

namespace Moonwell.Testing
{
    /// <summary>Cases that check buffs: their hooks and ticks, expiry, stacking, what is shown on the unit, and a unit ending.</summary>
    public static class BuffCases
    {
        /// <summary>Checks that the log is exactly the expected lines, in order.</summary>
        private static void ExpectLog(string[] expected, string what)
        {
            Expect.Equal(expected.Length, TestLog.Lines.Count, what + ": line count");
            for (var i = 0; i < expected.Length; i++)
            {
                Expect.Equal(expected[i], TestLog.Lines[i], what + ": line " + i);
            }
        }

        /// <summary>Checks that the ability calls are exactly the expected ones, in order.</summary>
        private static void ExpectCalls(FakeWorld world, string[] expected, string what)
        {
            Expect.Equal(expected.Length, world.AbilityCalls.Count, what + ": call count");
            for (var i = 0; i < expected.Length; i++)
            {
                Expect.Equal(expected[i], world.AbilityCalls[i], what + ": call " + i);
            }
        }

        /// <summary>How many lines of the log equal a line.</summary>
        private static int CountOf(string line)
        {
            var count = 0;
            for (var i = 0; i < TestLog.Lines.Count; i++)
            {
                if (TestLog.Lines[i] == line)
                {
                    count++;
                }
            }

            return count;
        }

        /// <summary>Registers a Striker as the target unit, and makes it.</summary>
        private static Unit Target(FakeWorld world)
        {
            ObjectTypes.Unit(ObjectTypes.FourCC("H001"), () => new Striker());
            return Unit.Of(world.NewUnit(ObjectTypes.FourCC("H001")))!;
        }

        /// <summary>A unit that is a source of buffs.</summary>
        private static Unit NewSource(FakeWorld world)
        {
            return Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
        }

        /// <summary>A buff is added, applied, ticks at each tick, then expires and is removed, in that order.</summary>
        public static void TicksThenExpiry(FakeWorld world)
        {
            var target = Target(world);
            TestLog.Lines.Clear();
            var buff = target.Apply(new Burn());
            world.Advance(3.1f);
            ExpectLog(new string[] { "added", "apply", "tick", "tick", "tick", "expire", "removed" }, "the log");
            Expect.True(buff.HasEnded, "the buff has ended");
        }

        /// <summary>Applying a buff that is already there refreshes it: the first buff stays, with the new source and duration.</summary>
        public static void Refresh(FakeWorld world)
        {
            var target = Target(world);
            var firstSource = NewSource(world);
            var secondSource = NewSource(world);
            TestLog.Lines.Clear();
            var first = target.Apply(new Burn(), firstSource);
            world.Advance(2f);
            var second = target.Apply(new Burn(), secondSource);
            Expect.Same(first, second, "the buff returned the second time");
            Expect.Equal(1, target.GetAll<Burn>().Count, "the burns on the unit");
            Expect.Same(secondSource, first.Source, "the source after the refresh");
            Expect.Equal(2, CountOf("apply"), "the applies logged");
            world.Advance(2.9f);
            Expect.True(!first.HasEnded, "the buff is still on at 4.9 seconds");
            world.Advance(0.2f);
            Expect.True(first.HasEnded, "the buff is gone at 5.1 seconds");
        }

        /// <summary>Separate stacking gives each apply its own buff.</summary>
        public static void Separate(FakeWorld world)
        {
            var target = Target(world);
            target.Apply(new SeparateBurn());
            target.Apply(new SeparateBurn());
            Expect.Equal(2, target.GetAll<SeparateBurn>().Count, "the separate burns on the unit");
        }

        /// <summary>Per-source stacking gives one buff per source: a second apply from the same source is refreshed.</summary>
        public static void PerSource(FakeWorld world)
        {
            var target = Target(world);
            var firstSource = NewSource(world);
            var secondSource = NewSource(world);
            target.Apply(new SourceBurn(), firstSource);
            target.Apply(new SourceBurn(), firstSource);
            Expect.Equal(1, target.GetAll<SourceBurn>().Count, "the burns from the first source");
            target.Apply(new SourceBurn(), secondSource);
            Expect.Equal(2, target.GetAll<SourceBurn>().Count, "the burns after the second source");
        }

        /// <summary>A registered buff shows through its carrier ability when the first one is on, and is removed with its carrier when the last one ends.</summary>
        public static void Display(FakeWorld world)
        {
            ObjectTypes.BuffDisplay<SeparateBurn>(ObjectTypes.FourCC("B000"), ObjectTypes.FourCC("A00B"));
            var target = Target(world);
            target.Apply(new SeparateBurn());
            target.Apply(new SeparateBurn());
            ExpectCalls(world, new string[] { "add " + ObjectTypes.FourCC("A00B"), "hide " + ObjectTypes.FourCC("A00B") }, "the calls while both are on");
            world.Advance(3.1f);
            ExpectCalls(world, new string[] { "add " + ObjectTypes.FourCC("A00B"), "hide " + ObjectTypes.FourCC("A00B"), "remove " + ObjectTypes.FourCC("A00B"), "remove " + ObjectTypes.FourCC("B000") }, "the calls after both have expired");
        }

        /// <summary>When the unit is removed, the buff is removed without expiring.</summary>
        public static void UnitEnds(FakeWorld world)
        {
            var target = Target(world);
            var buff = target.Apply(new Burn());
            TestLog.Lines.Clear();
            target.Remove();
            ExpectLog(new string[] { "removed" }, "the log");
            Expect.True(buff.HasEnded, "the buff has ended");
            Expect.True(target.IsGone, "the unit is gone");
        }

        /// <summary>A buff that applies itself again in OnExpire stays on with a fresh duration, and ends after the second expiry.</summary>
        public static void ReapplyInOnExpire(FakeWorld world)
        {
            var target = Target(world);
            var buff = target.Apply(new ReapplyBurn());
            world.Advance(3.1f);
            Expect.True(!buff.HasEnded, "the buff is still on after its first expiry");
            world.Advance(3f);
            Expect.True(buff.HasEnded, "the buff has ended after its second expiry");
        }

        /// <summary>A buff with no duration stays on its unit until it is removed.</summary>
        public static void UntilRemoved(FakeWorld world)
        {
            var target = Target(world);
            var buff = target.Apply(new Lasting());
            world.Advance(100f);
            Expect.True(!buff.HasEnded, "the buff is still on after 100 seconds");
        }

        /// <summary>A death ends a buff: the buff sees the death first, then it is removed.</summary>
        public static void EndsOnDeath(FakeWorld world)
        {
            var target = Target(world);
            var buff = target.Apply(new DyingBurn());
            TestLog.Lines.Clear();
            Inbound.Died(target.Handle, null);
            Expect.True(buff.HasEnded, "the buff has ended");
            var died = TestLog.Lines.IndexOf("burn died");
            Expect.True(died >= 0, "the buff saw the death");
            Expect.True(TestLog.Lines.IndexOf("removed") > died, "the buff was removed after it saw the death");
        }

        /// <summary>A buff whose class keeps it through death goes on ticking on the dead unit.</summary>
        public static void KeptThroughDeath(FakeWorld world)
        {
            var target = Target(world);
            var buff = target.Apply(new DeathlessBurn());
            Inbound.Died(target.Handle, null);
            TestLog.Lines.Clear();
            world.Advance(1.1f);
            Expect.True(!buff.HasEnded, "the buff is still on");
            ExpectLog(new string[] { "tick" }, "the log");
        }

        /// <summary>Applying a buff that a death ends makes the world deliver deaths; one kept through death does not.</summary>
        public static void ListensForDeath(FakeWorld world)
        {
            var target = NewSource(world);
            target.Apply(new DeathlessBurn());
            Expect.True(!world.Listening.Contains(GameEvent.Death), "deaths are not delivered for a buff kept through death");
            target.Apply(new Burn());
            Expect.True(world.Listening.Contains(GameEvent.Death), "deaths are delivered");
        }
    }
}
