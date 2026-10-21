using System;
using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>Lists the cases, runs them, and prints the results.</summary>
    public static class Cases
    {
        /// <summary>Every case, grouped by file in task order.</summary>
        public static List<Case> All()
        {
            var list = new List<Case>();
            AddFirst(list);
            AddMiddle(list);
            AddLast(list);
            return list;
        }

        /// <summary>Adds the first cases: the harness, collections, errors, ownership, timers, hooks, actors and objects (Lua allows 200 locals in a function, and each case made here is one).</summary>
        private static void AddFirst(List<Case> list)
        {
            list.Add(new Case { Name = "harness.handles-are-keys", Run = HarnessCases.HandlesAreKeys });
            list.Add(new Case { Name = "harness.expect-fails", Run = HarnessCases.ExpectFails });
            list.Add(new Case { Name = "collections.set-keeps-order", Run = CollectionCases.SetKeepsOrder });
            list.Add(new Case { Name = "collections.set-remove-keeps-order", Run = CollectionCases.SetRemoveKeepsOrder });
            list.Add(new Case { Name = "collections.set-snapshot", Run = CollectionCases.SetSnapshot });
            list.Add(new Case { Name = "collections.map-keeps-order", Run = CollectionCases.MapKeepsOrder });
            list.Add(new Case { Name = "collections.map-remove", Run = CollectionCases.MapRemove });
            list.Add(new Case { Name = "errors.log-line", Run = ErrorCases.LogLine });
            list.Add(new Case { Name = "errors.screen-line", Run = ErrorCases.ScreenLine });
            list.Add(new Case { Name = "errors.log-line-where", Run = ErrorCases.LogLineWhere });
            list.Add(new Case { Name = "errors.screen-line-where", Run = ErrorCases.ScreenLineWhere });
            list.Add(new Case { Name = "errors.shows-at", Run = ErrorCases.ShowsAt });
            list.Add(new Case { Name = "errors.report-throttles", Run = ErrorCases.ReportThrottles });
            list.Add(new Case { Name = "errors.report-many-keys", Run = ErrorCases.ReportManyKeys });
            list.Add(new Case { Name = "names.of", Run = ErrorCases.NamesOf });
            list.Add(new Case { Name = "names.class-of-type", Run = ErrorCases.ClassOfType });
            list.Add(new Case { Name = "world.advance", Run = ErrorCases.WorldAdvance });
            list.Add(new Case { Name = "ownership.ends-newest-first", Run = OwnershipCases.EndsNewestFirst });
            list.Add(new Case { Name = "ownership.ends-grandchildren", Run = OwnershipCases.EndsGrandchildren });
            list.Add(new Case { Name = "ownership.end-once", Run = OwnershipCases.EndOnce });
            list.Add(new Case { Name = "ownership.adopt-into-ended", Run = OwnershipCases.AdoptIntoEnded });
            list.Add(new Case { Name = "ownership.child-ends-alone", Run = OwnershipCases.ChildEndsAlone });
            list.Add(new Case { Name = "ownership.adopt-while-ending", Run = OwnershipCases.AdoptWhileEnding });
            list.Add(new Case { Name = "ownership.error-does-not-stop", Run = OwnershipCases.ErrorDoesNotStop });
            list.Add(new Case { Name = "ownership.run-sets-current", Run = OwnershipCases.RunSetsCurrent });
            list.Add(new Case { Name = "ownership.adopt-in-finish", Run = OwnershipCases.AdoptInFinish });
            list.Add(new Case { Name = "timers.after", Run = TimerCases.After });
            list.Add(new Case { Name = "timers.every", Run = TimerCases.Every });
            list.Add(new Case { Name = "timers.no-drift", Run = TimerCases.NoDrift });
            list.Add(new Case { Name = "timers.same-time-in-creation-order", Run = TimerCases.SameTimeInCreationOrder });
            list.Add(new Case { Name = "timers.end-cancels", Run = TimerCases.EndCancels });
            list.Add(new Case { Name = "timers.owned", Run = TimerCases.OwnerAdopts });
            list.Add(new Case { Name = "timers.current-in-callback", Run = TimerCases.CurrentInCallback });
            list.Add(new Case { Name = "timers.error-isolated", Run = TimerCases.ErrorIsolated });
            list.Add(new Case { Name = "timers.remaining", Run = TimerCases.Remaining });
            list.Add(new Case { Name = "hooks.overrides", Run = HookCases.Overrides });
            list.Add(new Case { Name = "hooks.cached", Run = HookCases.Cached });
            list.Add(new Case { Name = "hooks.damage-listener", Run = HookCases.DamageListener });
            list.Add(new Case { Name = "hooks.seen-once-per-class", Run = HookCases.SeenOncePerClass });
            list.Add(new Case { Name = "hooks.reset-clears", Run = HookCases.ResetClears });
            list.Add(new Case { Name = "events.need-listens-once", Run = HookCases.NeedListensOnce });
            list.Add(new Case { Name = "actors.add-and-get", Run = ActorCases.AddAndGet });
            list.Add(new Case { Name = "actors.remove", Run = ActorCases.RemoveOne });
            list.Add(new Case { Name = "actors.remove-all", Run = ActorCases.RemoveAllOf });
            list.Add(new Case { Name = "actors.current-in-onadded", Run = ActorCases.CurrentInOnAdded });
            list.Add(new Case { Name = "actors.end-removes-last-first", Run = ActorCases.EndRemovesLastFirst });
            list.Add(new Case { Name = "actors.add-to-ended", Run = ActorCases.AddToEnded });
            list.Add(new Case { Name = "actors.component-owns-timer", Run = ActorCases.ComponentOwnsTimer });
            list.Add(new Case { Name = "actors.error-in-onadded", Run = ActorCases.ErrorInOnAdded });
            list.Add(new Case { Name = "game.start-order", Run = ActorCases.GameStartOrder });
            list.Add(new Case { Name = "game.dispatch-removal", Run = ActorCases.GameDispatchRemoval });
            list.Add(new Case { Name = "players.get", Run = ActorCases.PlayersGet });
            list.Add(new Case { Name = "players.named", Run = ActorCases.PlayersNamed });
            list.Add(new Case { Name = "players.neutral", Run = ActorCases.PlayersNeutral });
            list.Add(new Case { Name = "players.start", Run = ActorCases.PlayersStart });
            list.Add(new Case { Name = "players.existing-out-of-range", Run = ActorCases.ExistingOutOfRange });
            list.Add(new Case { Name = "players.print", Run = PlayerCases.PlayersPrint });
            list.Add(new Case { Name = "players.print-seconds", Run = PlayerCases.PlayersPrintSeconds });
            list.Add(new Case { Name = "game.print", Run = PlayerCases.GamePrint });
        }

        /// <summary>Adds the player, local, unit and inventory cases (Lua allows 200 locals in a function, and each case made here is one).</summary>
        private static void AddMiddle(List<Case> list)
        {
            list.Add(new Case { Name = "players.gold", Run = PlayerCases.PlayersGold });
            list.Add(new Case { Name = "players.states", Run = PlayerCases.PlayersStates });
            list.Add(new Case { Name = "players.playing", Run = PlayerCases.PlayersPlaying });
            list.Add(new Case { Name = "players.slot-state", Run = PlayerCases.PlayersSlotState });
            list.Add(new Case { Name = "players.leave-order", Run = PlayerCases.PlayersLeaveOrder });
            list.Add(new Case { Name = "players.leave-listened-when-reacted", Run = PlayerCases.PlayersLeaveListenedWhenReacted });
            list.Add(new Case { Name = "players.name", Run = PlayerCases.PlayersName });
            list.Add(new Case { Name = "players.color", Run = PlayerCases.PlayersColor });
            list.Add(new Case { Name = "players.team", Run = PlayerCases.PlayersTeam });
            list.Add(new Case { Name = "players.race", Run = PlayerCases.PlayersRace });
            list.Add(new Case { Name = "players.alliance", Run = PlayerCases.PlayersAlliance });
            list.Add(new Case { Name = "players.allied", Run = PlayerCases.PlayersAllied });
            list.Add(new Case { Name = "players.tech", Run = PlayerCases.PlayersTech });
            list.Add(new Case { Name = "players.ability-available", Run = PlayerCases.PlayersAbilityAvailable });
            list.Add(new Case { Name = "local.runs-for-local-player", Run = LocalCases.RunsForLocalPlayer });
            list.Add(new Case { Name = "local.skips-other-players", Run = LocalCases.SkipsOtherPlayers });
            list.Add(new Case { Name = "local.reads-keep-no-last-value", Run = LocalCases.ReadsKeepNoLastValue });
            list.Add(new Case { Name = "local.throw-leaves-reads-kept", Run = LocalCases.ThrowLeavesReadsKept });
            list.Add(new Case { Name = "map.components-in-order", Run = MapCases.ComponentsInOrder });
            list.Add(new Case { Name = "map.components-not-added-early", Run = MapCases.ComponentsNotAddedEarly });
            list.Add(new Case { Name = "map.components-assigned", Run = MapCases.ComponentsAssigned });
            list.Add(new Case { Name = "map.components-add-many", Run = MapCases.ComponentsAddMany });
            list.Add(new Case { Name = "units.subclass-assigning-drops-parent", Run = MapCases.SubclassAssigningDropsParent });
            list.Add(new Case { Name = "units.subclass-add-many-keeps-parent", Run = MapCases.SubclassAddManyKeepsParent });
            list.Add(new Case { Name = "units.create-default-facing", Run = MapCases.CreateDefaultFacing });
            list.Add(new Case { Name = "map.settings-default-empty", Run = MapCases.SettingsDefaultEmpty });
            list.Add(new Case { Name = "map.balance-default", Run = MapCases.BalanceDefault });
            list.Add(new Case { Name = "types.fourcc", Run = UnitCases.FourCC });
            list.Add(new Case { Name = "types.duplicate", Run = UnitCases.Duplicate });
            list.Add(new Case { Name = "types.idof", Run = UnitCases.IdOf });
            list.Add(new Case { Name = "objects.upgrade-level", Run = ObjectKindCases.UpgradeLevel });
            list.Add(new Case { Name = "objects.id-twice", Run = ObjectKindCases.IdTwice });
            list.Add(new Case { Name = "objects.register-before-components", Run = ObjectKindCases.RegisterBeforeComponents });
            list.Add(new Case { Name = "units.registered-class", Run = UnitCases.RegisteredClass });
            list.Add(new Case { Name = "units.fallback-class", Run = UnitCases.FallbackClass });
            list.Add(new Case { Name = "units.components-before-enter", Run = UnitCases.ComponentsBeforeEnter });
            list.Add(new Case { Name = "units.sweep-detects-removal", Run = UnitCases.SweepDetectsRemoval });
            list.Add(new Case { Name = "units.remove", Run = UnitCases.Remove });
            list.Add(new Case { Name = "units.owned", Run = UnitCases.OwnedByOwner });
            list.Add(new Case { Name = "units.not-owned-by-current", Run = UnitCases.NotOwnedByCurrent });
            list.Add(new Case { Name = "units.create-generic", Run = UnitCases.CreateGeneric });
            list.Add(new Case { Name = "units.sweep-continues-after-error", Run = UnitCases.SweepContinuesAfterError });
            list.Add(new Case { Name = "units.add-in-onleave", Run = UnitCases.AddInOnLeave });
            list.Add(new Case { Name = "units.enter", Run = UnitCases.Enter });
            list.Add(new Case { Name = "units.player-after-removal", Run = UnitCases.PlayerAfterRemoval });
            list.Add(new Case { Name = "units.dictionary-key", Run = UnitCases.DictionaryKey });
            list.Add(new Case { Name = "units.life-and-mana", Run = UnitCases.LifeAndMana });
            list.Add(new Case { Name = "units.position", Run = UnitCases.Position });
            list.Add(new Case { Name = "units.move-to", Run = UnitCases.MoveTo });
            list.Add(new Case { Name = "units.facing", Run = UnitCases.Facing });
            list.Add(new Case { Name = "units.level", Run = UnitCases.Level });
            list.Add(new Case { Name = "units.kill", Run = UnitCases.Kill });
            list.Add(new Case { Name = "units.kill-gone", Run = UnitCases.KillGone });
            list.Add(new Case { Name = "units.pause", Run = UnitCases.Pause });
            list.Add(new Case { Name = "units.scale", Run = UnitCases.Scale });
            list.Add(new Case { Name = "units.tint", Run = UnitCases.Tint });
            list.Add(new Case { Name = "units.fly-height", Run = UnitCases.FlyHeight });
            list.Add(new Case { Name = "units.gone-reads-last", Run = UnitCases.GoneReadsLast });
            list.Add(new Case { Name = "units.fly-height-ground", Run = UnitCases.FlyHeightGroundUnit });
            list.Add(new Case { Name = "units.max-clamps", Run = UnitCases.MaxClampsLifeAndMana });
            list.Add(new Case { Name = "units.max-last-values", Run = UnitCases.MaxLastValues });
            list.Add(new Case { Name = "units.fly-height-has-amrf", Run = UnitCases.FlyHeightWithAmrf });
            list.Add(new Case { Name = "units.paused-gone-before-sweep", Run = UnitCases.PausedGoneBeforeSweep });
            list.Add(new Case { Name = "units.gone-before-sweep", Run = UnitCases.GoneBeforeSweep });
            list.Add(new Case { Name = "units.gone-before-sweep-local", Run = UnitCases.GoneBeforeSweepInLocal });
            list.Add(new Case { Name = "units.order-immediate", Run = UnitCases.OrderImmediate });
            list.Add(new Case { Name = "units.order-point", Run = UnitCases.OrderPoint });
            list.Add(new Case { Name = "units.order-target", Run = UnitCases.OrderTarget });
            list.Add(new Case { Name = "units.order-gone", Run = UnitCases.OrderGone });
            list.Add(new Case { Name = "units.inventory", Run = ItemCases.Inventory });
            list.Add(new Case { Name = "units.inventory-limits", Run = ItemCases.InventoryLimits });
            list.Add(new Case { Name = "units.inventory-gone", Run = ItemCases.InventoryGone });
        }

        /// <summary>Adds the hero, item, destructable, damage, ability and buff cases, and the rest.</summary>
        private static void AddLast(List<Case> list)
        {
            list.Add(new Case { Name = "heroes.unregistered-is-hero", Run = HeroCases.UnregisteredIsHero });
            list.Add(new Case { Name = "heroes.experience", Run = HeroCases.Experience });
            list.Add(new Case { Name = "heroes.level", Run = HeroCases.Level });
            list.Add(new Case { Name = "heroes.stats", Run = HeroCases.Stats });
            list.Add(new Case { Name = "heroes.skill-points", Run = HeroCases.SkillPoints });
            list.Add(new Case { Name = "heroes.revive", Run = HeroCases.Revive });
            list.Add(new Case { Name = "heroes.gone", Run = HeroCases.Gone });
            list.Add(new Case { Name = "heroes.gone-before-sweep", Run = HeroCases.GoneBeforeSweep });
            list.Add(new Case { Name = "items.registered-class", Run = ItemCases.RegisteredClass });
            list.Add(new Case { Name = "items.fallback-class", Run = ItemCases.FallbackClass });
            list.Add(new Case { Name = "items.pickup-order", Run = ItemCases.PickupOrder });
            list.Add(new Case { Name = "items.drop-order", Run = ItemCases.DropOrder });
            list.Add(new Case { Name = "items.listen", Run = ItemCases.Listens });
            list.Add(new Case { Name = "items.sweep", Run = ItemCases.SweepDetectsRemoval });
            list.Add(new Case { Name = "items.owned-create", Run = ItemCases.OwnedByOwner });
            list.Add(new Case { Name = "items.charges", Run = ItemCases.Charges });
            list.Add(new Case { Name = "items.player", Run = ItemCases.PlayerOf });
            list.Add(new Case { Name = "items.position", Run = ItemCases.Position });
            list.Add(new Case { Name = "items.carrier", Run = ItemCases.Carrier });
            list.Add(new Case { Name = "items.carrier-in-hooks", Run = ItemCases.CarrierInHooks });
            list.Add(new Case { Name = "items.carrier-dies", Run = ItemCases.CarrierDies });
            list.Add(new Case { Name = "items.carrier-unseen", Run = ItemCases.CarrierUnseen });
            list.Add(new Case { Name = "items.carrier-slot-move", Run = ItemCases.CarrierSlotMove });
            list.Add(new Case { Name = "items.carrier-hero-dies", Run = ItemCases.CarrierHeroDies });
            list.Add(new Case { Name = "items.carrier-nonhero-dies", Run = ItemCases.CarrierNonHeroDiesSilently });
            list.Add(new Case { Name = "items.place-while-carried", Run = ItemCases.PlaceWhileCarried });
            list.Add(new Case { Name = "items.removed-unit-drops-after", Run = ItemCases.RemovedUnitDropsAfterTheCall });
            list.Add(new Case { Name = "items.give-full-inventory", Run = ItemCases.GiveFullInventory });
            list.Add(new Case { Name = "items.give-held-again", Run = ItemCases.GiveHeldItemAgain });
            list.Add(new Case { Name = "items.freed-slot-first", Run = ItemCases.FreedSlotFirst });
            list.Add(new Case { Name = "items.player-gone-before-read", Run = ItemCases.PlayerGoneBeforeRead });
            list.Add(new Case { Name = "items.gone-reads-last", Run = ItemCases.ItemGoneReadsLast });
            list.Add(new Case { Name = "items.gone-before-sweep", Run = ItemCases.GoneBeforeSweep });
            list.Add(new Case { Name = "destructables.watch", Run = DestructableCases.Watch });
            list.Add(new Case { Name = "destructables.sweep", Run = DestructableCases.SweepDetectsRemoval });
            list.Add(new Case { Name = "damage.order", Run = DamageCases.Order });
            list.Add(new Case { Name = "damage.change-amount", Run = DamageCases.ChangeAmount });
            list.Add(new Case { Name = "damage.unknown-units", Run = DamageCases.UnknownUnits });
            list.Add(new Case { Name = "damage.listen", Run = DamageCases.Listen });
            list.Add(new Case { Name = "damage.nested", Run = DamageCases.Nested });
            list.Add(new Case { Name = "damage.actor-ends-mid-dispatch", Run = DamageCases.ActorEndsMidDispatch });
            list.Add(new Case { Name = "damage.error-isolated", Run = DamageCases.ErrorIsolated });
            list.Add(new Case { Name = "damage-package.no-trigger-without-need", Run = InstallCases.NoTriggerWithoutNeed });
            list.Add(new Case { Name = "damage-package.listens-for-an-interface", Run = InstallCases.ListensForAnInterface });
            list.Add(new Case { Name = "damage-package.listens-for-on-damaged", Run = InstallCases.ListensForOnDamaged });
            list.Add(new Case { Name = "damage-package.order", Run = StageCases.Order });
            list.Add(new Case { Name = "damage-package.before-armor-changes", Run = StageCases.BeforeArmorChanges });
            list.Add(new Case { Name = "damage-package.armor-for-one-hit", Run = StageCases.ArmorForOneHit });
            list.Add(new Case { Name = "damage-package.after-armor-changes", Run = StageCases.AfterArmorChanges });
            list.Add(new Case { Name = "damage-package.type-set-after-armor-reported", Run = StageCases.TypeSetAfterArmorReported });
            list.Add(new Case { Name = "damage-package.removed-mid-stage", Run = StageCases.RemovedMidStage });
            list.Add(new Case { Name = "damage-package.melee-or-ranged", Run = StageCases.MeleeOrRanged });
            list.Add(new Case { Name = "damage-package.attack-type-change", Run = StageCases.AttackTypeChange });
            list.Add(new Case { Name = "damage-package.defense-type-for-one-hit", Run = StageCases.DefenseTypeForOneHit });
            list.Add(new Case { Name = "damage-package.attack-type-reads-late", Run = StageCases.AttackTypeReadsLate });
            list.Add(new Case { Name = "damage-package.lethal-spare", Run = LandingCases.LethalSpare });
            list.Add(new Case { Name = "damage-package.lethal-only-when-lethal", Run = LandingCases.LethalOnlyWhenLethal });
            list.Add(new Case { Name = "damage-package.landed-after-zero", Run = LandingCases.LandedAfterZero });
            list.Add(new Case { Name = "damage-package.burst-counts", Run = LandingCases.BurstCounts });
            list.Add(new Case { Name = "damage-package.immunity-stops", Run = LandingCases.ImmunityStops });
            list.Add(new Case { Name = "damage-package.immunity-after-type-change", Run = LandingCases.ImmunityAfterTypeChange });
            list.Add(new Case { Name = "damage-package.immunity-inside-hit", Run = LandingCases.ImmunityInsideHit });
            list.Add(new Case { Name = "damage-package.nested-keeps-outer", Run = LandingCases.NestedKeepsOuter });
            list.Add(new Case { Name = "damage-package.stray-open-ended", Run = LandingCases.StrayOpenEnded });
            list.Add(new Case { Name = "damage-package.deal-returns-hit", Run = DealCases.DealReturnsHit });
            list.Add(new Case { Name = "damage-package.deal-attack-type", Run = DealCases.DealAttackType });
            list.Add(new Case { Name = "damage-package.deal-starts-listening", Run = DealCases.DealStartsListening });
            list.Add(new Case { Name = "damage-package.deal-inside-reaction-nests", Run = DealCases.DealInsideReactionNests });
            list.Add(new Case { Name = "damage-package.chain-cut", Run = DealCases.ChainCut });
            list.Add(new Case { Name = "damage-package.chain-across-frames", Run = DealCases.ChainAcrossFrames });
            list.Add(new Case { Name = "damage-package.chain-through-on-damaged", Run = DealCases.ChainThroughOnDamaged });
            list.Add(new Case { Name = "death.order", Run = DeathCases.Order });
            list.Add(new Case { Name = "death.self", Run = DeathCases.Self });
            list.Add(new Case { Name = "death.not-an-end", Run = DeathCases.NotAnEnd });
            list.Add(new Case { Name = "abilities.registered", Run = AbilityCases.Registered });
            list.Add(new Case { Name = "abilities.generic", Run = AbilityCases.Generic });
            list.Add(new Case { Name = "cast.order", Run = AbilityCases.Order });
            list.Add(new Case { Name = "cast.fields", Run = AbilityCases.Fields });
            list.Add(new Case { Name = "cast.owns-what-it-starts", Run = AbilityCases.OwnsWhatItStarts });
            list.Add(new Case { Name = "cast.next-ends-previous", Run = AbilityCases.NextEndsPrevious });
            list.Add(new Case { Name = "cast.listen", Run = AbilityCases.Listens });
            list.Add(new Case { Name = "buff.ticks-then-expiry", Run = BuffCases.TicksThenExpiry });
            list.Add(new Case { Name = "buff.refresh", Run = BuffCases.Refresh });
            list.Add(new Case { Name = "buff.separate", Run = BuffCases.Separate });
            list.Add(new Case { Name = "buff.per-source", Run = BuffCases.PerSource });
            list.Add(new Case { Name = "buff.display", Run = BuffCases.Display });
            list.Add(new Case { Name = "buff.unit-ends", Run = BuffCases.UnitEnds });
            list.Add(new Case { Name = "buff.reapply-in-onexpire", Run = BuffCases.ReapplyInOnExpire });
            list.Add(new Case { Name = "buff.until-removed", Run = BuffCases.UntilRemoved });
            list.Add(new Case { Name = "buff.ends-on-death", Run = BuffCases.EndsOnDeath });
            list.Add(new Case { Name = "buff.kept-through-death", Run = BuffCases.KeptThroughDeath });
            list.Add(new Case { Name = "buff.listens-for-death", Run = BuffCases.ListensForDeath });
            list.Add(new Case { Name = "floats.whole-literal", Run = FloatCases.WholeLiteral });
            list.Add(new Case { Name = "floats.no-wrap", Run = FloatCases.NoWrap });
            list.Add(new Case { Name = "floats.folded", Run = FloatCases.Folded });
            list.Add(new Case { Name = "floats.stored", Run = FloatCases.Stored });
            list.Add(new Case { Name = "floats.converted", Run = FloatCases.Converted });
            list.Add(new Case { Name = "floats.bare", Run = FloatCases.Bare });
            list.Add(new Case { Name = "floats.ints-stay", Run = FloatCases.IntsStay });
        }

        /// <summary>Runs every case on a fresh world, and returns a PASS or FAIL line for each, then DONE.</summary>
        public static List<string> RunAll()
        {
            var lines = new List<string>();
            var cases = All();
            foreach (var testCase in cases)
            {
                var world = FakeWorld.Install();
                try
                {
                    testCase.Run(world);
                    lines.Add("PASS " + testCase.Name);
                }
                catch (Exception error)
                {
                    lines.Add("FAIL " + testCase.Name + ": " + error.Message);
                }
            }

            lines.Add("DONE " + cases.Count);
            return lines;
        }

        /// <summary>The map's entry point: prints every line of RunAll.</summary>
        public static void Main()
        {
            foreach (var line in RunAll())
            {
                Emit(line);
            }
        }

        // Writes one line of output; in Lua this is the print global, which the Lua test host captures.
        /// @CSharpLua.Template = "print({0})"
        public static void Emit(string line)
        {
            Console.WriteLine(line);
        }
    }
}
