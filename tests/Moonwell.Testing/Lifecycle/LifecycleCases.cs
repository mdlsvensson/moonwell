using System;
using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>Cases for the full audit's fixes to core's lifecycle: what Moonwell takes in, who owns what, how objects end, and the guards a release build keeps.</summary>
    public static class LifecycleCases
    {
        /// <summary>Moves the game on by a number of the interactions package's steps, one step at a time.</summary>
        private static void Steps(FakeWorld world, int count)
        {
            for (var i = 0; i < count; i++)
            {
                world.Advance(Interactions.Step);
            }
        }

        /// <summary>Makes a footman at a place in the world, which Moonwell has not taken in.</summary>
        private static unit FootmanAt(FakeWorld world, float x, float y)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            world.MoveUnit(handle, x, y);
            world.PlacedUnits.Add(handle);
            return handle;
        }

        /// <summary>A footman the game makes (trained, summoned, made by a native) of a type with no class enters beside an aura: the aura meets it and Find finds it. A unit already in the world that Moonwell never took in is taken in when units are first asked for.</summary>
        public static void UnitsTheGameMakesAreMet(FakeWorld world)
        {
            Interactions.Install(new FakeInteractionWorld());
            TestLog.Lines.Clear();
            var before = FootmanAt(world, -100.0f, 0.0f);
            var holder = Unit.Of(FootmanAt(world, 0.0f, 0.0f))!;
            var aura = new FireAura(300.0f, 0.125f);
            aura.Tag = unit => unit.Handle == before ? "before" : "trained";
            holder.Interact(aura);
            Expect.True(KnownUnits.Find(before) != null, "the unit already in the world is taken in at the first ask for units");

            var trained = FootmanAt(world, 100.0f, 0.0f);
            Inbound.Entered(trained);
            Expect.True(KnownUnits.Find(trained) != null, "the trained unit is taken in as it enters");
            Steps(world, 40);
            var found = Find.InRange<Unit>(0.0f, 0.0f, 300.0f);
            Expect.Equal(3, found.Count, "the units Find finds: the holder, the one from before and the trained one");
            Expect.True(TestLog.Lines.Contains("meet before") && TestLog.Lines.Contains("meet trained"), "the aura met both: " + string.Join(", ", TestLog.Lines));
        }

        /// <summary>Code inside Local.For throws on one machine only: the error is reported there, and the synced code after the block runs on every machine, so the game stays the same on each.</summary>
        public static void LocalThrowKeepsSync(FakeWorld world)
        {
            var gold = new int[2];
            for (var machine = 0; machine < 2; machine++)
            {
                world.LocalSlot = machine;
                var player = Player.Get(1);
                player.Gold = 0;
                Timer.After(1.0f, () =>
                {
                    Local.For(Player.Get(0), () => { throw new InvalidOperationException("only on player 1's machine"); });
                    player.Gold += 10;
                });
                world.Advance(1.0f);
                gold[machine] = player.Gold;
            }

            Expect.Equal("10 10", gold[0] + " " + gold[1], "player 2's gold on player 1's machine and on player 2's");
            var reported = 0;
            foreach (var line in world.Logged)
            {
                if (line.Contains("Local.For failed: only on player 1's machine"))
                {
                    reported++;
                }
            }

            Expect.Equal(1, reported, "the error's reports, on player 1's machine only: " + string.Join(" | ", world.Logged));
        }
    }
}
