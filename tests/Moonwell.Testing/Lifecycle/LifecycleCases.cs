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

        /// <summary>A Mirror Image of a hero of a class enters with the hero's type id (seen 10-26): it is taken in as a plain unit, a Hero and not the class, with none of the class's starting components and no OnCreated, and IsIllusion reads true; OnUnitCreated still sees it (decision 77).</summary>
        public static void IllusionIsPlain(FakeWorld world)
        {
            CountedHero.Created = 0;
            var id = ObjectTypes.FourCC("H0IL");
            ObjectTypes.Unit(id, () => new CountedHero());
            var watch = Game.Instance.Add(new CreatedWatch());
            var hero = Unit.Of(world.NewUnit(id, hero: true))!;
            var image = world.NewUnit(id, hero: true);
            world.MakeIllusion(image);
            Inbound.Entered(image);
            var taken = KnownUnits.Find(image);
            Expect.True(taken != null, "the illusion is taken in");
            Expect.Equal("Hero " + id + " True 0", Names.NameOf(taken!) + " " + taken!.TypeId + " " + taken.IsIllusion + " " + taken.ComponentCount, "the illusion's class, type id, IsIllusion and components");
            Expect.Equal(1, CountedHero.Created, "OnCreated's runs: the hero's only");
            Expect.True(!hero.IsIllusion && hero is CountedHero && hero.ComponentCount == 1, "the hero itself is of its class, with its component, and no illusion");
            Expect.True(watch.Seen.Contains(taken), "OnUnitCreated sees the illusion");
        }

        /// <summary>The lines of the log that hold a text.</summary>
        private static int Logged(FakeWorld world, string text)
        {
            var count = 0;
            foreach (var line in world.Logged)
            {
                if (line.Contains(text))
                {
                    count++;
                }
            }

            return count;
        }

        /// <summary>One component added to a second unit is refused, in every build: it is reported and stays on the first, so ending the first leaves the second's timer running (decision 74).</summary>
        public static void SharedComponentRefused(FakeWorld world)
        {
            var a = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            var b = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            var ran = 0;
            b.Run(() => { Timer.Every(1.0f, () => { ran++; }); });
            var shared = a.Add(new Marker());
            try
            {
                b.Add(shared);
            }
            catch (MoonwellException error)
            {
                Expect.True(false, "the second add threw: " + error.Message);
            }

            Expect.Equal(1, Logged(world, "Unit.Add failed: Marker is already on an actor or has ended"), "the reports of the second add: " + string.Join(" | ", world.Logged));
            Expect.True(shared.Actor == a && !b.Has<Marker>(), "the component stays on the first unit only");
            a.Remove();
            world.Advance(2.5f);
            Expect.Equal("2 False 1", ran + " " + b.HasEnded + " " + b.OwnedCount, "runs of the second unit's timer after the first was removed, whether it ended, and what it owns");
        }

        /// <summary>A Separate buff that applies itself again in OnExpire is refused, in every build: it is reported, the buff starts nothing again and ends at its expiry, and its unit ends (in a release build it used to be linked twice, and its unit's end never returned) (decision 74).</summary>
        public static void BuffReappliedToItselfRefused(FakeWorld world)
        {
            var target = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            var buff = target.Apply(new SelfApplying());
            world.Advance(1.1f);
            Expect.Equal(1, Logged(world, "Unit.Apply failed: SelfApplying is already on an actor or has ended"), "the reports of the apply: " + string.Join(" | ", world.Logged));
            Expect.True(buff.HasEnded, "the buff has ended at its expiry");
            Expect.Equal(1, buff.Applied, "the buff's applies");
            target.Remove();
            Expect.True(target.HasEnded, "the unit has ended");
            Expect.Equal(0, target.OwnedCount, "what the ended unit still owns");
        }

        /// <summary>Removing a component from another actor than the one it is on is refused, in every build: it is reported, and the component stays on its actor (decision 74).</summary>
        public static void RemoveOthersComponentRefused(FakeWorld world)
        {
            var a = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            var b = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            var marker = a.Add(new Marker());
            try
            {
                b.Remove(marker);
            }
            catch (MoonwellException error)
            {
                Expect.True(false, "the remove threw: " + error.Message);
            }

            Expect.Equal(1, Logged(world, "Unit.Remove failed: Marker is not on this actor"), "the reports of the remove: " + string.Join(" | ", world.Logged));
            Expect.True(!marker.HasEnded && a.Has<Marker>(), "the component stays on its unit");
        }

        /// <summary>A hero of a class carrying an item Moonwell knows is removed with Remove(). The game fires the item's drop after RemoveUnit, in the same frame, while the hero's type id still reads (seen 10-26): its handle makes no second object, and no drop reaction runs for it; OnCreated, OnUnitCreated and OnLeave each run once.</summary>
        public static void RemovedHeroDropMakesNoSecond(FakeWorld world)
        {
            CountedHero.Created = 0;
            CountedHero.Left = 0;
            var id = ObjectTypes.FourCC("H0RM");
            ObjectTypes.Unit(id, () => new CountedHero());
            var watch = Game.Instance.Add(new CreatedWatch());
            var handle = world.NewUnit(id, hero: true);
            Inbound.Entered(handle);
            var hero = (CountedHero)KnownUnits.Find(handle)!;
            ObjectTypes.Item(ObjectTypes.FourCC("I0RM"), () => new CarrierWatch());
            var item = (CarrierWatch)Item.Of(world.NewItem(ObjectTypes.FourCC("I0RM")))!;
            hero.Give(item);
            hero.Remove();
            Expect.True(world.UnitTypeId(handle) != 0, "the removed hero's type id still reads in the frame of its removal");
            Expect.True(Unit.Of(handle) == null, "Unit.Of of the removed hero's handle in that frame");

            world.Advance(0.01f);
            Expect.True(KnownUnits.Find(handle) == null && Unit.Of(handle) == null, "no second object for the removed hero after the drop");
            Expect.Equal("1 1 1", CountedHero.Created + " " + watch.Seen.Count + " " + CountedHero.Left, "OnCreated's, OnUnitCreated's and OnLeave's runs");
            Expect.True(!item.DropSeen, "the item's drop reaction for the removed hero");
            world.Advance(0.3f);
            Expect.Equal(1, CountedHero.Left, "OnLeave's runs after the sweep");
        }

        /// <summary>A unit of a class made with Unit.Create and removed in the same instant: the game's enter event for it comes after CreateUnit returns, in the same frame, reading its type id (seen 10-26). It makes no second object.</summary>
        public static void CreatedAndRemovedEnterMakesNoSecond(FakeWorld world)
        {
            CountedHero.Created = 0;
            CountedHero.Left = 0;
            var id = ObjectTypes.FourCC("H0CR");
            ObjectTypes.Unit(id, () => new CountedHero());
            var watch = Game.Instance.Add(new CreatedWatch());
            var made = Unit.Create<CountedHero>(Player.Get(0), 0.0f, 0.0f);
            var handle = made.Handle;
            made.Remove();
            Inbound.Entered(handle);
            Expect.True(KnownUnits.Find(handle) == null && Unit.Of(handle) == null, "no second object for the removed unit after its enter");
            world.Advance(0.3f);
            Expect.Equal("1 1 1", CountedHero.Created + " " + watch.Seen.Count + " " + CountedHero.Left, "OnCreated's, OnUnitCreated's and OnLeave's runs");
        }

        /// <summary>A hero's experience and level are lowered as the game allows: SetHeroXP and SetHeroLevel only raise (seen 10-21), so a lower experience goes through AddHeroXP with a negative amount, which keeps the level, and a lower level through UnitStripHeroLevel, as blizzard.j's SetHeroLevelBJ does. The last values are the game's, so a removed hero reads what it had.</summary>
        public static void HeroLowered(FakeWorld world)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC("Hpal"), hero: true);
            var hero = (Hero)Unit.Of(handle)!;
            hero.Experience = 500;
            hero.Experience -= 200;
            Expect.Equal(300, hero.Experience, "the experience after it was lowered by 200");
            hero.SetLevel(5);
            hero.SetLevel(2, false);
            Expect.Equal(2, hero.Level, "the level after it was lowered to 2");
            hero.SetLevel(2);
            Expect.Equal("experience 500 True|add-experience -200 True|level 5 True|strip 3", string.Join("|", world.HeroCalls), "the calls that changed the hero");
            world.SetExperience(handle, 100, true);
            Expect.Equal(300, world.ExperienceOf(handle), "the fake world's experience after a lower SetHeroXP, as the game's");
            hero.Remove();
            Expect.Equal("300 2", hero.Experience + " " + hero.Level, "the removed hero's last experience and level");
        }

        /// <summary>A component with a helper named like a reaction and taking other parameters (OnDeath(string)) and with no reaction: adding it works on .NET as in Lua, its mask has no OnDeath, and the unit's death does not call the helper. Only a method that overrides the base's reaction counts.</summary>
        public static void HelperNamedLikeReaction(FakeWorld world)
        {
            Helpered.Log = "";
            var unit = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            var added = "added";
            Helpered? helpered = null;
            try
            {
                helpered = unit.Add(new Helpered());
            }
            catch (Exception error)
            {
                added = "Add threw: " + error.Message;
            }

            Expect.Equal("added", added, "adding the component");
            Expect.True((helpered!.Mask & Hook.OnDeath) == 0, "the component's mask has no OnDeath");
            Runtime.World.Listen(Moonwell.Hosting.GameEvent.Death);
            unit.Kill();
            Expect.Equal("", Helpered.Log, "the helper's calls after the death");
        }

        /// <summary>A component whose OnDamaged hides the base's (new, not override) has no OnDamaged reaction, on .NET as in Lua, and is not called.</summary>
        public static void HidingMethodIsNoReaction(FakeWorld world)
        {
            Hiding.Calls = 0;
            var unit = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            var hiding = unit.Add(new Hiding());
            Expect.True((hiding.Mask & Hook.OnDamaged) == 0, "the component's mask has no OnDamaged");
            Inbound.Damaged(unit.Handle, null, 10.0f, true);
            Expect.Equal(0, Hiding.Calls, "the hiding method's calls after a damage");
        }

        /// <summary>OnUnitCreated runs for a unit as it comes into the game (it enters, Unit.Create makes it, or it is there at the start), not when Moonwell takes it in later: a unit that came in while nothing wanted OnUnitCreated, first hit two minutes later, and one taken in when units are first asked for, run none.</summary>
        public static void UnitCreatedOnlyWhenItComesIn(FakeWorld world)
        {
            var old = FootmanAt(world, 0.0f, 0.0f);
            Inbound.Entered(old);
            var source = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo"), slot: 1))!;
            world.Advance(60.0f);
            var watch = Game.Instance.Add(new CreatedWatch());
            var unseen = FootmanAt(world, 50.0f, 0.0f);
            world.Advance(60.0f);
            Inbound.Damaged(old, source.Handle, 10.0f, true);
            Expect.True(KnownUnits.Find(old) != null, "the old unit is taken in when it is first hit");
            Interactions.Install(new FakeInteractionWorld());
            Find.InRange<Unit>(0.0f, 0.0f, 100.0f);
            Expect.True(KnownUnits.Find(unseen) != null, "the unit already in the world is taken in when units are first asked for");
            Expect.Equal(0, watch.Seen.Count, "OnUnitCreated's runs for units taken in long after they came in");

            var entering = FootmanAt(world, 0.0f, 50.0f);
            Inbound.Entered(entering);
            var made = Unit.Create(ObjectTypes.FourCC("hfoo"), Player.Get(0), 0.0f, 0.0f);
            Expect.Equal(2, watch.Seen.Count, "OnUnitCreated's runs for a unit that enters and one Unit.Create makes");
            Expect.True(watch.Seen[0] == KnownUnits.Find(entering) && watch.Seen[1] == made, "the units OnUnitCreated was given");
        }

        /// <summary>A unit and an item of a class created for an owner that has already ended are made in the game and end at once: they run neither OnCreated nor OnLeave, so code that pairs the two stays even.</summary>
        public static void EndedOwnerRunsNeitherCreatedNorLeave(FakeWorld world)
        {
            CountedHero.Created = 0;
            CountedHero.Left = 0;
            CountedItem.Created = 0;
            CountedItem.Left = 0;
            ObjectTypes.Unit(ObjectTypes.FourCC("H0EO"), () => new CountedHero());
            ObjectTypes.Item(ObjectTypes.FourCC("I0EO"), () => new CountedItem());
            var owner = Game.Instance.Add(new CreatedWatch());
            owner.End();
            var unit = Unit.Create<CountedHero>(Player.Get(0), 0.0f, 0.0f, owner: owner);
            var item = Item.Create<CountedItem>(0.0f, 0.0f, owner: owner);
            Expect.True(unit.HasEnded && item.HasEnded, "the unit and the item have ended");
            Expect.True(world.RemovedUnits.Contains(unit.Handle) && world.RemovedItems.Contains(item.Handle), "the unit and the item are removed from the game");
            Expect.Equal("0 0 0 0", CountedHero.Created + " " + CountedHero.Left + " " + CountedItem.Created + " " + CountedItem.Left, "the unit's OnCreated and OnLeave runs, and the item's");
        }

        /// <summary>A buff's hidden carrier, which shows its icon, is made permanent when it is added, so a morph (Metamorphosis, Bear Form), which takes away an ability code added that is not permanent (seen 10-26), leaves it on the unit.</summary>
        public static void BuffCarrierSurvivesMorph(FakeWorld world)
        {
            var buffId = ObjectTypes.FourCC("B0CH");
            var carrier = ObjectTypes.FourCC("A0CH");
            ObjectTypes.BuffDisplay<Chilled>(buffId, carrier);
            var handle = world.NewUnit(ObjectTypes.FourCC("Edem"), hero: true);
            var target = Unit.Of(handle)!;
            target.Apply(new Chilled(0.0f, 0.5f));
            world.Morph(handle);
            Expect.True(world.HasAddedAbility(handle, carrier), "the buff's carrier after the morph: " + string.Join(", ", world.AbilityCalls));
        }

        /// <summary>A death ends the buffs that were on the unit as it died, after they have seen it; a buff the death's own reactions apply has not seen the death and stays.</summary>
        public static void BuffAppliedInDeathStays(FakeWorld world)
        {
            ObjectTypes.Unit(ObjectTypes.FourCC("h0MO"), () => new Mourned());
            var unit = (Mourned)Unit.Of(world.NewUnit(ObjectTypes.FourCC("h0MO")))!;
            var before = unit.Apply(new Lasting());
            Inbound.Died(unit.Handle, null);
            Expect.True(before.HasEnded, "the buff that was on before the death has ended");
            Expect.True(unit.Mark != null && !unit.Mark.HasEnded, "the buff OnDeath applied stays");
        }

        /// <summary>A timer adopted by a second owner is refused, in every build: it is reported and stays with its first owner (decision 74).</summary>
        public static void AdoptAgainRefused(FakeWorld world)
        {
            var first = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            var second = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            var ran = 0;
            var timer = Timer.Every(1.0f, () => { ran++; }, first);
            try
            {
                second.Adopt(timer);
            }
            catch (MoonwellException error)
            {
                Expect.True(false, "the adopt threw: " + error.Message);
            }

            Expect.Equal(1, Logged(world, "Unit.Adopt failed: Timer already has an owner or has ended"), "the reports of the adopt: " + string.Join(" | ", world.Logged));
            second.Remove();
            world.Advance(2.5f);
            Expect.Equal("2 1 0", ran + " " + first.OwnedCount + " " + second.OwnedCount, "the timer's runs after the second owner was removed, and what each owns");
        }
    }
}
