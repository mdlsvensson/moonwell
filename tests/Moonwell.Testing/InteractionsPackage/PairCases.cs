using System.Collections.Generic;
using Moonwell.Engine;

namespace Moonwell.Testing
{
    /// <summary>Cases for the pairs of an interaction, on the default settings: a step of 1/32 second, so a FireAura's tick of half a second is 16 steps.</summary>
    public static class PairCases
    {
        /// <summary>Installs a fake interactions world and clears the test log.</summary>
        private static void Fresh()
        {
            Interactions.Install(new FakeInteractionWorld());
            TestLog.Lines.Clear();
        }

        /// <summary>Moves the game on by a number of steps, one step at a time.</summary>
        private static void Steps(FakeWorld world, int count)
        {
            for (var i = 0; i < count; i++)
            {
                world.Advance(Interactions.Step);
            }
        }

        /// <summary>Makes a unit of the game at a place and takes it in.</summary>
        private static Unit UnitAt(FakeWorld world, float x, float y)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            world.MoveUnit(handle, x, y);
            return Unit.Of(handle)!;
        }

        /// <summary>Checks that the test log is exactly the expected lines, in order.</summary>
        private static void ExpectLog(string[] expected, string what)
        {
            Expect.Equal(expected.Length, TestLog.Lines.Count, what + ": line count");
            for (var i = 0; i < expected.Length; i++)
            {
                Expect.Equal(expected[i], TestLog.Lines[i], what + ": line " + i);
            }
        }

        /// <summary>Wait(1f) in an OnTick skips the pair's ticks for a second: with a tick of a quarter second, the ticks at 16, 24 and 32 steps are skipped and the one at 40 runs.</summary>
        public static void Wait(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            UnitAt(world, 100.0f, 0.0f);
            var aura = new FireAura(300.0f, 0.25f);
            var waited = false;
            aura.Ticking = pair =>
            {
                if (!waited)
                {
                    waited = true;
                    pair.Wait(1.0f);
                }
            };
            a.Interact(aura, Game.Instance);
            var start = Tracking.StepNow;
            Steps(world, 8);
            Expect.Equal(1, aura.TickSteps.Count, "the tick at the meeting");
            Steps(world, 31);
            Expect.Equal(1, aura.TickSteps.Count, "no tick for a second");
            Expect.Equal(1, aura.Pairs.Count, "the pair is kept");
            Steps(world, 1);
            Expect.Equal(2, aura.TickSteps.Count, "a tick after the second");
            Expect.Equal(start + 40, aura.TickSteps[1], "at step 40");
            Steps(world, 8);
            Expect.Equal(start + 48, aura.TickSteps[2], "then every tick again");
        }

        /// <summary>TryCooldown is true and starts the cooldown, false while it runs, then true again; Cooldown gives the seconds left.</summary>
        public static void TryCooldown(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            UnitAt(world, 100.0f, 0.0f);
            var aura = new FireAura(300.0f, Interactions.Step);
            var tries = new List<bool>();
            var left = new List<float>();
            aura.Ticking = pair =>
            {
                tries.Add(pair.TryCooldown(0.25f));
                left.Add(pair.Cooldown);
            };
            a.Interact(aura, Game.Instance);
            Steps(world, 10);
            Expect.Equal(10, tries.Count, "a tick at every step");
            Expect.True(tries[0], "true at first");
            for (var i = 1; i < 8; i++)
            {
                Expect.True(!tries[i], "false for the quarter second, try " + i);
            }

            Expect.True(tries[8], "true again after it");
            Expect.True(!tries[9], "and false again after that");
            Expect.Near(0.25f, left[0], "a quarter second left when it starts");
            Expect.Near(0.125f, left[4], "an eighth left four steps later");
            Expect.Near(0.25f, left[8], "a quarter second again when it starts again");
        }

        /// <summary>A paused pair does not tick but stays, resumed it ticks again, and paused it still parts.</summary>
        public static void PauseResume(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var b = UnitAt(world, 100.0f, 0.0f);
            var aura = new FireAura();
            aura.Tag = unit => unit == b ? "b" : "?";
            a.Interact(aura, Game.Instance);
            Steps(world, 16);
            var pair = aura.Pairs[0];
            pair.Pause();
            Expect.True(pair.IsPaused, "paused");
            Steps(world, 32);
            Expect.Equal(1, aura.TickSteps.Count, "no ticks while paused");
            Expect.Equal(1, aura.Pairs.Count, "the pair stays");
            pair.Resume();
            Expect.True(!pair.IsPaused, "resumed");
            Steps(world, 16);
            Expect.Equal(2, aura.TickSteps.Count, "it ticks again");
            pair.Pause();
            world.MoveUnit(b.Handle, 600.0f, 0.0f);
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "tick b", "tick b", "part b" }, "a paused pair still parts");
            Expect.True(pair.HasEnded, "and has ended");
        }

        /// <summary>Stop parts the pair at once, and the two do not meet again until the other has been out of range.</summary>
        public static void Stop(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var b = UnitAt(world, 100.0f, 0.0f);
            var aura = new FireAura();
            aura.Tag = unit => unit == b ? "b" : "?";
            a.Interact(aura, Game.Instance);
            Steps(world, 16);
            var pair = aura.Pairs[0];
            pair.Stop();
            ExpectLog(new string[] { "meet b", "tick b", "part b" }, "Stop parts at once");
            Expect.True(pair.HasEnded, "the pair has ended");
            Expect.Equal(0, aura.Pairs.Count, "no pair");
            Steps(world, 32);
            Expect.Equal(3, TestLog.Lines.Count, "b in range does not meet again");
            Expect.Equal(1, aura.Asked, "nor is it asked about");
            world.MoveUnit(b.Handle, 600.0f, 0.0f);
            Steps(world, 16);
            world.MoveUnit(b.Handle, 100.0f, 0.0f);
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "tick b", "part b", "meet b", "tick b" }, "out of range once, it meets again");
            Expect.True(aura.Pairs[0] != pair, "as a new pair");
        }

        /// <summary>Data is kept from OnMeet to OnPart; Since counts from the meeting; the places, the distance and the angle are those read at the check.</summary>
        public static void DataAndSince(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var b = UnitAt(world, 0.0f, 200.0f);
            var aura = new FireAura();
            var token = new List<int>();
            object? parted = null;
            var since = new List<float>();
            var seen = new List<object?>();
            aura.Meeting = pair => pair.Data = token;
            aura.Ticking = pair =>
            {
                since.Add(pair.Since);
                seen.Add(pair.Data);
            };
            aura.Parting = pair => parted = pair.Data;
            a.Interact(aura, Game.Instance);
            Steps(world, 16);
            var met = aura.Pairs[0];
            Expect.Near(0.0f, since[0], "Since is 0 at the meeting");
            Expect.Near(0.0f, met.HolderX, "HolderX");
            Expect.Near(0.0f, met.HolderY, "HolderY");
            Expect.Near(0.0f, met.OtherX, "OtherX");
            Expect.Near(200.0f, met.OtherY, "OtherY");
            Expect.Near(200.0f, met.Distance, "Distance");
            Expect.Near(90.0f, met.Angle, "Angle from the holder to the other");
            world.MoveUnit(b.Handle, 0.0f, 250.0f);
            Steps(world, 16);
            Expect.Near(0.5f, since[1], "half a second at the next tick");
            Expect.Same(token, seen[1], "Data kept");
            Expect.Near(250.0f, met.OtherY, "OtherY read at this check");
            Expect.Near(250.0f, met.Distance, "and the distance with it");
            Expect.Near(0.5f, met.Since, "Since read outside the check");
            world.MoveUnit(b.Handle, 0.0f, 600.0f);
            Steps(world, 16);
            Expect.Same(token, parted, "Data in OnPart");
        }

        /// <summary>A pair belongs to its interaction; its reactions run with it as the current owner, so a timer made in OnMeet is the pair's and ends when it parts.</summary>
        public static void PairOwns(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var b = UnitAt(world, 100.0f, 0.0f);
            var aura = new FireAura();
            Timer? made = null;
            Scope? meeting = null;
            Scope? ticking = null;
            Scope? parting = null;
            aura.Meeting = pair =>
            {
                meeting = Scope.Current;
                made = pair.After(100.0f, () => { });
            };
            aura.Ticking = pair => ticking = Scope.Current;
            aura.Parting = pair => parting = Scope.Current;
            a.Interact(aura, Game.Instance);
            Steps(world, 16);
            var met = aura.Pairs[0];
            Expect.Same(aura, met.Parent, "the pair is its interaction's");
            Expect.Same(met, meeting, "OnMeet runs with the pair as the current owner");
            Expect.Same(met, ticking, "and OnTick");
            Expect.Same(met, made!.Parent, "the timer is the pair's");
            Expect.True(!made!.HasEnded, "and runs");
            world.MoveUnit(b.Handle, 600.0f, 0.0f);
            Steps(world, 16);
            Expect.Same(met, parting, "OnPart runs with the pair as the current owner");
            Expect.True(made!.HasEnded, "the timer ended with the pair");
            Expect.True(Scope.Current == Runtime.Root, "and the root is current after");
        }

        /// <summary>After in an interaction's OnMeet is the interaction's; the pair's After is the pair's.</summary>
        public static void PairAfterIsTheInteractions(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            UnitAt(world, 100.0f, 0.0f);
            var aura = new TimedAura();
            a.Interact(aura, Game.Instance);
            Steps(world, 16);
            var met = aura.Pairs[0];
            Expect.Same(aura, aura.Mine!.Parent, "After in OnMeet is the interaction's");
            Expect.Same(met, aura.Paired!.Parent, "the pair's After is the pair's");
        }

        /// <summary>An interaction ends with its scope or its holder, whichever ends first, and each pair parts once.</summary>
        public static void InteractEndsWithScopeOrHolder(FakeWorld world)
        {
            Fresh();
            var u = UnitAt(world, 0.0f, 0.0f);
            UnitAt(world, 100.0f, 0.0f);
            var scope = new TestScope("scope", new List<string>());
            var first = new FireAura();
            var second = new FireAura();
            var firstParts = 0;
            first.Parting = pair => { firstParts++; };
            u.Interact(first, scope);
            u.Interact(second, Game.Instance);
            Steps(world, 16);
            Expect.Equal(1, first.Pairs.Count, "the first met the other unit");
            scope.End();
            Expect.True(first.HasEnded, "the first ended with its scope");
            Expect.Equal(1, firstParts, "its pair parted once");
            Expect.True(!second.HasEnded, "the second goes on");
            u.Remove();
            Steps(world, 16);
            Expect.True(second.HasEnded, "the second ended with its holder");
            Expect.Equal(1, firstParts, "and the first's pair did not part again");
        }

        /// <summary>After in an interaction's OnPart is the interaction's, so it runs after the pair has parted.</summary>
        public static void PartAfterRuns(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var b = UnitAt(world, 100.0f, 0.0f);
            var aura = new TimedAura();
            a.Interact(aura, Game.Instance);
            Steps(world, 16);
            world.MoveUnit(b.Handle, 600.0f, 0.0f);
            Steps(world, 16);
            Expect.Same(aura, aura.Parted!.Parent, "After in OnPart is the interaction's");
            Steps(world, 40);
            Expect.Equal(1, aura.PartRan, "and it ran after the parting");
        }
    }
}
