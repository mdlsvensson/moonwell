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
    }
}
