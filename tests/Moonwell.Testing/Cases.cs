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
            list.Add(new Case { Name = "players.gold", Run = PlayerCases.PlayersGold });
            list.Add(new Case { Name = "players.states", Run = PlayerCases.PlayersStates });
            list.Add(new Case { Name = "local.runs-for-local-player", Run = LocalCases.RunsForLocalPlayer });
            list.Add(new Case { Name = "local.skips-other-players", Run = LocalCases.SkipsOtherPlayers });
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
            list.Add(new Case { Name = "items.registered-class", Run = ItemCases.RegisteredClass });
            list.Add(new Case { Name = "items.fallback-class", Run = ItemCases.FallbackClass });
            list.Add(new Case { Name = "items.pickup-order", Run = ItemCases.PickupOrder });
            list.Add(new Case { Name = "items.drop-order", Run = ItemCases.DropOrder });
            list.Add(new Case { Name = "items.listen", Run = ItemCases.Listens });
            list.Add(new Case { Name = "items.sweep", Run = ItemCases.SweepDetectsRemoval });
            list.Add(new Case { Name = "items.owned-create", Run = ItemCases.OwnedByOwner });
            list.Add(new Case { Name = "destructables.watch", Run = DestructableCases.Watch });
            list.Add(new Case { Name = "destructables.sweep", Run = DestructableCases.SweepDetectsRemoval });
            list.Add(new Case { Name = "damage.order", Run = DamageCases.Order });
            list.Add(new Case { Name = "damage.change-amount", Run = DamageCases.ChangeAmount });
            list.Add(new Case { Name = "damage.unknown-units", Run = DamageCases.UnknownUnits });
            list.Add(new Case { Name = "damage.listen", Run = DamageCases.Listen });
            list.Add(new Case { Name = "damage.nested", Run = DamageCases.Nested });
            list.Add(new Case { Name = "damage.actor-ends-mid-dispatch", Run = DamageCases.ActorEndsMidDispatch });
            list.Add(new Case { Name = "damage.error-isolated", Run = DamageCases.ErrorIsolated });
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
            return list;
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
