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

        /// <summary>A creep's own OnDeath starts a 120 s respawn timer; its corpse is gone 90 s later. What OnDeath makes belongs to the game, so the respawn runs; a timer it names the unit the owner of ends with the corpse (decision 64).</summary>
        public static void DeathRespawnOutlivesCorpse(FakeWorld world)
        {
            ObjectTypes.Unit(ObjectTypes.FourCC("n0RS"), () => new Respawning());
            var creep = (Respawning)Unit.Of(world.NewUnit(ObjectTypes.FourCC("n0RS"), slot: 24))!;
            creep.Kill();
            world.Advance(90.1f);
            world.VanishUnit(creep.Handle);
            world.Advance(40.0f);
            Expect.True(creep.HasEnded, "the creep has ended with its corpse");
            Expect.Equal("1 90", creep.Respawned + " " + creep.Lingered, "the respawn timer's runs, and the runs of the timer the creep owns");
        }

        /// <summary>A 5 s chill applied over a 2 s one, 1 s in, refreshes it: the buff there takes the new duration from the refresh, and keeps its own strength and what it counted (decision 66). A refresh with no duration lasts until the buff is removed.</summary>
        public static void RefreshTakesNewDuration(FakeWorld world)
        {
            var target = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            var first = target.Apply(new Chilled(2.0f, 0.3f));
            world.Advance(1.0f);
            var refreshed = target.Apply(new Chilled(5.0f, 0.6f));
            Expect.Same(first, refreshed, "the buff the refresh gives");
            world.Advance(2.5f);
            Expect.True(!first.HasEnded, "the buff at 3.5 s, refreshed at 1 s for 5 s, has not ended");
            Expect.Near(5.0f, first.Duration, "the buff's duration");
            Expect.Near(0.3f, first.Strength, "the buff's strength");
            Expect.Equal(2, first.Applied, "the applies the buff counted");
            world.Advance(2.6f);
            Expect.True(first.HasEnded, "the buff at 6.1 s has ended");

            var lasting = target.Apply(new Chilled(2.0f, 0.3f));
            target.Apply(new Chilled(0.0f, 0.6f));
            world.Advance(10.0f);
            Expect.True(!lasting.HasEnded, "a buff refreshed with no duration lasts until it is removed");
        }

        /// <summary>A buff an aura's pair applies in OnMeet belongs to the unit it is on, not to the pair or the aura: it stays after they part, unless OnPart removes it, as the guide's FrostAura does (decision 65).</summary>
        public static void PairBuffBelongsToUnit(FakeWorld world)
        {
            Interactions.Install(new FakeInteractionWorld());
            TestLog.Lines.Clear();
            var holder = Unit.Of(FootmanAt(world, 0.0f, 0.0f))!;
            var kept = Unit.Of(FootmanAt(world, 100.0f, 0.0f))!;
            var removed = Unit.Of(FootmanAt(world, 0.0f, 100.0f))!;
            var aura = new FireAura(300.0f, 0.125f);
            aura.Meeting = pair => { pair.Other.Apply(new Chilled(0.0f, 0.5f)); };
            aura.Parting = pair =>
            {
                if (pair.Other == removed)
                {
                    pair.Other.Remove<Chilled>();
                }
            };
            holder.Interact(aura);
            Steps(world, 8);
            Expect.True(kept.Has<Chilled>() && removed.Has<Chilled>(), "both others are chilled while near");
            Expect.Same(kept, kept.Get<Chilled>()!.Owner, "the owner of the buff the pair applied");

            world.MoveUnit(kept.Handle, 900.0f, 900.0f);
            world.MoveUnit(removed.Handle, -900.0f, -900.0f);
            Steps(world, 40);
            Expect.Equal(2, TestLog.Lines.FindAll(line => line.StartsWith("part")).Count, "the pairs that parted: " + string.Join(", ", TestLog.Lines));
            Expect.True(kept.Has<Chilled>(), "the buff stays on the unit after its pair parted");
            Expect.True(!removed.Has<Chilled>(), "the buff OnPart removed");
        }
    }
}
