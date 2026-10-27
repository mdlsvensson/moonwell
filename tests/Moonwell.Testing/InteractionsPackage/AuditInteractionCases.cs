using System.Collections.Generic;
using Moonwell.Engine;

namespace Moonwell.Testing
{
    /// <summary>Cases for the full audit's fixes to the interactions package, on the default settings: a step of 1/32 second and a cell check of 0.1 second.</summary>
    public static class AuditInteractionCases
    {
        /// <summary>Installs a fake interactions world and clears the test log.</summary>
        private static FakeInteractionWorld Fresh()
        {
            var fake = new FakeInteractionWorld();
            Interactions.Install(fake);
            TestLog.Lines.Clear();
            return fake;
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

        /// <summary>Makes units in a row far from where a case works, so the cell checks take more than a step to come round to its units.</summary>
        private static void Crowd(FakeWorld world, int count)
        {
            for (var i = 0; i < count; i++)
            {
                UnitAt(world, (float)(i * 10), 0.0f);
            }
        }

        /// <summary>How many lines of the test log equal a line.</summary>
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

        /// <summary>Checks that the test log is exactly the expected lines, in order.</summary>
        private static void ExpectLog(string[] expected, string what)
        {
            Expect.Equal(expected.Length, TestLog.Lines.Count, what + ": line count");
            for (var i = 0; i < expected.Length; i++)
            {
                Expect.Equal(expected[i], TestLog.Lines[i], what + ": line " + i);
            }
        }

        /// <summary>Whether a line of the game's log has the text.</summary>
        private static bool Logged(FakeWorld world, string text)
        {
            foreach (var line in world.Logged)
            {
                if (line.Contains(text))
                {
                    return true;
                }
            }

            return false;
        }

        /// <summary>A holder and its other moved far together in one jump (Mass Teleport, a map's teleport) are in range at every check: the pair does not part and meet again, and ticks on. The moved other is found at its new place within a cell check.</summary>
        public static void TeleportTogetherStaysPaired(FakeWorld world)
        {
            Fresh();
            Crowd(world, 30);
            var a = UnitAt(world, -800.0f, -800.0f);
            var b = UnitAt(world, -700.0f, -800.0f);
            var aura = new FireAura(300.0f, 0.03125f);
            aura.Tag = unit => unit == b ? "b" : "?";
            a.Interact(aura);
            Steps(world, 4);
            Expect.Equal(1, aura.Pairs.Count, "a and b have met");
            var pair = aura.Pairs[0];
            TestLog.Lines.Clear();
            world.MoveUnit(a.Handle, 800.0f, 800.0f);
            world.MoveUnit(b.Handle, 900.0f, 800.0f);
            Steps(world, 8);
            Expect.Equal(0, CountOf("part b"), "b, in range at every check, does not part");
            Expect.Equal(0, CountOf("meet b"), "nor meet again");
            Expect.Equal(8, CountOf("tick b"), "it ticks at every step");
            Expect.Same(pair, aura.Pairs[0], "the pair is the same");
            Expect.Near(900.0f, pair.OtherX, "its reads are of the new place");
            Expect.Equal(1, Find.InRange<Unit>(900.0f, 800.0f, 50.0f).Count, "within a cell check Find finds b at its new place");
        }

        /// <summary>A pair stopped (the two meet again only once the other has been out of range), then holder and other moved far together: the other was never out of range, so it does not meet again.</summary>
        public static void TeleportTogetherKeepsStop(FakeWorld world)
        {
            Fresh();
            Crowd(world, 30);
            var a = UnitAt(world, -800.0f, -800.0f);
            var b = UnitAt(world, -700.0f, -800.0f);
            var aura = new FireAura(300.0f, 0.03125f);
            aura.Tag = unit => unit == b ? "b" : "?";
            a.Interact(aura);
            Steps(world, 4);
            aura.Pairs[0].Stop();
            Steps(world, 4);
            TestLog.Lines.Clear();
            world.MoveUnit(a.Handle, 800.0f, 800.0f);
            world.MoveUnit(b.Handle, 900.0f, 800.0f);
            Steps(world, 8);
            Expect.Equal(0, CountOf("meet b"), "the stopped other, never out of range, does not meet again");
            world.MoveUnit(b.Handle, -700.0f, -800.0f);
            Steps(world, 8);
            world.MoveUnit(b.Handle, 900.0f, 800.0f);
            Steps(world, 8);
            Expect.Equal(1, CountOf("meet b"), "out of range once, it meets again");
        }

        /// <summary>A holder moved far away alone parts with its other at its next check, once; the other's direct read does not keep a pair that is apart.</summary>
        public static void TeleportApartParts(FakeWorld world)
        {
            Fresh();
            Crowd(world, 30);
            var a = UnitAt(world, -800.0f, -800.0f);
            var b = UnitAt(world, -700.0f, -800.0f);
            var aura = new FireAura(300.0f, 0.03125f);
            aura.Tag = unit => unit == b ? "b" : "?";
            a.Interact(aura);
            Steps(world, 4);
            TestLog.Lines.Clear();
            world.MoveUnit(a.Handle, 800.0f, 800.0f);
            Steps(world, 1);
            Expect.Equal(1, CountOf("part b"), "b parts at the next check");
            Expect.Equal(0, aura.Pairs.Count, "no pair");
            Steps(world, 8);
            Expect.Equal(1, CountOf("part b"), "once");
            Expect.Equal(0, CountOf("meet b"), "and does not meet again");
        }

        /// <summary>The guide's FireAura burns an enemy; the unit changes sides (Charm, Possession, a new owner) and is no longer the holder's enemy: at the next tick Accepts says no, the pair parts and the burning stops. Back with the enemy, it is accepted at the next tick and burns again.</summary>
        public static void GuideFireAuraSparesChangedOwner(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var handle = world.NewUnit(ObjectTypes.FourCC("hfoo"), 1);
            world.MoveUnit(handle, 100.0f, 0.0f);
            var b = Unit.Of(handle)!;
            var aura = new GuideFireAura();
            aura.Tag = unit => unit == b ? "b" : "?";
            Expect.True(aura.RecheckAccepts, "Accepts is asked again unless the interaction says not");
            a.Interact(aura);
            Steps(world, 32);
            ExpectLog(new string[] { "meet b", "burn b", "burn b" }, "b, an enemy, burns");
            Expect.Equal(2, world.Dealt.Count, "twice");
            b.Player = a.Player;
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "burn b", "burn b", "part b" }, "charmed: the pair parts at the next tick");
            Steps(world, 32);
            Expect.Equal(2, world.Dealt.Count, "and b is burnt no more");
            Expect.Equal(0, aura.Pairs.Count, "no pair");
            b.Player = Player.Get(1);
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "burn b", "burn b", "part b", "meet b", "burn b" }, "an enemy again, b meets again at the next tick");
            Expect.Equal(3, world.Dealt.Count, "and burns");
        }

        /// <summary>With RecheckAccepts off, Accepts is asked only before a meeting, as before decision 73: the charmed unit burns on.</summary>
        public static void RecheckOffKeepsPair(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var handle = world.NewUnit(ObjectTypes.FourCC("hfoo"), 1);
            world.MoveUnit(handle, 100.0f, 0.0f);
            var b = Unit.Of(handle)!;
            var aura = new GuideFireAura(false);
            aura.Tag = unit => unit == b ? "b" : "?";
            Expect.True(!aura.RecheckAccepts, "turned off");
            a.Interact(aura);
            Steps(world, 16);
            b.Player = a.Player;
            Steps(world, 32);
            ExpectLog(new string[] { "meet b", "burn b", "burn b", "burn b" }, "the pair is kept and burns on");
        }

        /// <summary>A pair that Accepts refuses at a tick parts (OnPart once), and the other is not asked again in that check; it is asked again at the next tick, as any other in range, and meets again once accepted. Accepts runs with the interaction as the current owner, and a paused pair is asked too, as it still parts.</summary>
        public static void RecheckRefusalPartsAndMeetsAgain(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var b = UnitAt(world, 100.0f, 0.0f);
            var aura = new FireAura();
            aura.Tag = unit => unit == b ? "b" : "?";
            var accept = true;
            aura.Accepting = unit => accept;
            a.Interact(aura);
            Steps(world, 16);
            Expect.Equal(1, aura.Asked, "asked before the meeting");
            Steps(world, 16);
            Expect.Equal(2, aura.Asked, "and again before the next tick");
            ExpectLog(new string[] { "meet b", "tick b", "tick b" }, "accepted, it ticks");
            aura.AskedAs = null;
            accept = false;
            Steps(world, 16);
            Expect.Equal(3, aura.Asked, "refused once in the check, not asked again in it");
            Expect.Same(aura, aura.AskedAs, "with the interaction as the current owner");
            ExpectLog(new string[] { "meet b", "tick b", "tick b", "part b" }, "the refused pair parts, with no tick");
            Steps(world, 16);
            Expect.Equal(4, aura.Asked, "asked again at the next tick");
            Expect.Equal(4, TestLog.Lines.Count, "and refused, no meeting");
            accept = true;
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "tick b", "tick b", "part b", "meet b", "tick b" }, "accepted again, it meets again");
            aura.Pairs[0].Pause();
            accept = false;
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "tick b", "tick b", "part b", "meet b", "tick b", "part b" }, "a paused pair refused parts too");
        }

        /// <summary>An Accepts that throws at a tick is reported and refuses, so the pair parts once; asked again at the next tick it throws again and no meeting comes.</summary>
        public static void RecheckThrowingAcceptsRefuses(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var b = UnitAt(world, 100.0f, 0.0f);
            var aura = new FireAura();
            aura.Tag = unit => unit == b ? "b" : "?";
            var throws = false;
            aura.Accepting = unit =>
            {
                if (throws)
                {
                    throw new System.InvalidOperationException("no");
                }

                return true;
            };
            a.Interact(aura);
            Steps(world, 16);
            throws = true;
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "tick b", "part b" }, "the Accepts that threw refused: the pair parted");
            Expect.True(Logged(world, "FireAura.Accepts failed"), "the error is reported");
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "tick b", "part b" }, "asked again, it threw again: no meeting");
            Expect.Equal(3, aura.Asked, "asked once at each check");
            Expect.Same(Runtime.Root, Owner.Current, "the root is the current owner after the step");
        }

        /// <summary>OnPart runs once per OnMeet when Accepts, asked again at a tick, ends a pair itself, ends its other, or ends the interaction: each pair parts once, and the check stops when the interaction ends.</summary>
        public static void RecheckPartsOncePerMeet(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var b = UnitAt(world, 100.0f, 0.0f);
            var c = UnitAt(world, 0.0f, 100.0f);
            var d = UnitAt(world, 200.0f, 0.0f);
            var aura = new FireAura();
            aura.Tag = unit => unit == b ? "b" : unit == c ? "c" : unit == d ? "d" : "?";
            var round = 0;
            aura.Accepting = unit =>
            {
                if (round == 1 && unit == b)
                {
                    // Ends b's own pair from inside Accepts, then refuses: one part.
                    aura.Pairs[0].End();
                    return false;
                }

                if (round == 1 && unit == c)
                {
                    // Ends the other: its pairs part as it ends, and Accepts says yes, too late.
                    c.Remove();
                    return true;
                }

                if (round == 2)
                {
                    aura.End();
                    return false;
                }

                return true;
            };
            a.Interact(aura);
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "tick b", "meet c", "tick c", "meet d", "tick d" }, "three pairs");
            round = 1;
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "tick b", "meet c", "tick c", "meet d", "tick d", "part b", "part c", "tick d" }, "b's pair ended in Accepts parts once; c ended parts once; d ticks");
            round = 2;
            Steps(world, 16);
            Expect.True(aura.HasEnded, "the interaction ended in Accepts");
            Expect.Equal(1, CountOf("part d"), "d parts once");
            Expect.Equal(1, CountOf("part b"), "b parted once in all");
            Expect.Equal(1, CountOf("part c"), "c parted once in all");
            Expect.Equal(CountOf("meet b") + CountOf("meet c") + CountOf("meet d"), CountOf("part b") + CountOf("part c") + CountOf("part d"), "OnPart once per OnMeet");
        }
    }
}
