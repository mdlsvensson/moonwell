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
            AddMaths(list);
            AddInteractions(list);
            AddTerrain(list);
            AddAreas(list);
            AddInput(list);
            AddDebug(list);
            return list;
        }

        /// <summary>Adds the debug mode's cases; they exist only in a development build, as the debug mode does.</summary>
        private static void AddDebug(List<Case> list)
        {
            // Every case here must be named debug.*: the release Lua test leaves those out of its count by that prefix.
#if DEBUG
            list.Add(new Case { Name = "debug.clock.halt", Run = ClockCases.Halt });
            list.Add(new Case { Name = "debug.clock.step-advances", Run = ClockCases.StepAdvances });
            list.Add(new Case { Name = "debug.clock.speed", Run = ClockCases.Speed });
            list.Add(new Case { Name = "debug.clock.no-jump", Run = ClockCases.NoJump });
            list.Add(new Case { Name = "debug.clock.timer-while-halted", Run = ClockCases.TimerWhileHalted });
            list.Add(new Case { Name = "debug.clock.halt-inside-timer", Run = ClockCases.HaltInsideTimer });
            list.Add(new Case { Name = "debug.clock.catchup", Run = ClockCases.Catchup });
            list.Add(new Case { Name = "debug.session.off-at-start", Run = SessionCases.OffAtStart });
            list.Add(new Case { Name = "debug.session.toggle", Run = SessionCases.Toggle });
            list.Add(new Case { Name = "debug.session.halt-holds", Run = SessionCases.HaltHolds });
            list.Add(new Case { Name = "debug.session.halt-free", Run = SessionCases.HaltFree });
            list.Add(new Case { Name = "debug.session.map-paused-stays", Run = SessionCases.MapPausedStays });
            list.Add(new Case { Name = "debug.session.new-unit-held", Run = SessionCases.NewUnitHeld });
            list.Add(new Case { Name = "debug.session.step-free", Run = SessionCases.StepFree });
            list.Add(new Case { Name = "debug.session.step-held", Run = SessionCases.StepHeld });
            list.Add(new Case { Name = "debug.session.step-dropped", Run = SessionCases.StepDropped });
            list.Add(new Case { Name = "debug.session.resume-mid-step", Run = SessionCases.ResumeMidStep });
            list.Add(new Case { Name = "debug.session.speeds", Run = SessionCases.Speeds });
            list.Add(new Case { Name = "debug.session.step-count", Run = SessionCases.StepCount });
            list.Add(new Case { Name = "debug.session.off-restores", Run = SessionCases.OffRestores });
            list.Add(new Case { Name = "debug.session.select", Run = SessionCases.Select });
            list.Add(new Case { Name = "debug.session.select-next", Run = SessionCases.SelectNext });
            list.Add(new Case { Name = "debug.session.select-nothing", Run = SessionCases.SelectNothing });
            list.Add(new Case { Name = "debug.session.facade", Run = SessionCases.Facade });
            list.Add(new Case { Name = "debug.session.halt-from-code-turns-on", Run = SessionCases.HaltFromCodeTurnsOn });
            list.Add(new Case { Name = "debug.session.held-h-toggles-once", Run = SessionCases.HeldHTogglesOnce });
            list.Add(new Case { Name = "debug.session.held-unit-removed", Run = SessionCases.HeldUnitRemoved });
            list.Add(new Case { Name = "debug.panel.status", Run = PanelCases.Status });
            list.Add(new Case { Name = "debug.panel.unit", Run = PanelCases.SelectedUnit });
            list.Add(new Case { Name = "debug.panel.numbers", Run = PanelCases.Numbers });
            list.Add(new Case { Name = "debug.panel.cap", Run = PanelCases.Cap });
            list.Add(new Case { Name = "debug.panel.throwing-shown", Run = PanelCases.ThrowingShown });
            list.Add(new Case { Name = "debug.panel.refresh", Run = PanelCases.Refresh });
            list.Add(new Case { Name = "debug.panel.ended", Run = PanelCases.Ended });
            list.Add(new Case { Name = "debug.panel.off", Run = PanelCases.Off });
            list.Add(new Case { Name = "debug.panel.reuse", Run = PanelCases.Reuse });
            list.Add(new Case { Name = "debug.panel.stats", Run = PanelCases.Stats });
            list.Add(new Case { Name = "debug.interactions.candidates-no-tracking", Run = DebugViewCases.CandidatesNoTracking });
            list.Add(new Case { Name = "debug.interactions.lines", Run = DebugViewCases.Lines });
            list.Add(new Case { Name = "debug.interactions.paired-by", Run = DebugViewCases.PairedBy });
            list.Add(new Case { Name = "debug.interactions.draw", Run = DebugViewCases.Draw });
            list.Add(new Case { Name = "debug.interactions.pair-limit", Run = DebugViewCases.PairLimit });
            list.Add(new Case { Name = "debug.interactions.gizmo-ends", Run = DebugViewCases.GizmoEnds });
            list.Add(new Case { Name = "debug.interactions.stats", Run = DebugViewCases.Stats });
            list.Add(new Case { Name = "debug.interactions.step-length", Run = DebugViewCases.StepLength });
            list.Add(new Case { Name = "debug.areas.lines", Run = AreasViewCases.Lines });
            list.Add(new Case { Name = "debug.areas.outlines", Run = AreasViewCases.Outlines });
            list.Add(new Case { Name = "debug.areas.not-in-any", Run = AreasViewCases.NotInAny });
            list.Add(new Case { Name = "debug.areas.region", Run = AreasViewCases.Region });
            list.Add(new Case { Name = "debug.areas.no-tracker", Run = AreasViewCases.NoTracker });
            list.Add(new Case { Name = "debug.areas.registered", Run = AreasViewCases.Registered });
#endif
        }

        /// <summary>Adds the interactions package's cases.</summary>
        private static void AddInteractions(List<Case> list)
        {
            list.Add(new Case { Name = "interactions.grid.cells", Run = InteractionsGridCases.Cells });
            list.Add(new Case { Name = "interactions.grid.move", Run = InteractionsGridCases.Move });
            list.Add(new Case { Name = "interactions.grid.collect", Run = InteractionsGridCases.Collect });
            list.Add(new Case { Name = "interactions.grid.edge", Run = InteractionsGridCases.Edge });
            list.Add(new Case { Name = "interactions.grid.lazy", Run = InteractionsGridCases.Lazy });
            list.Add(new Case { Name = "interactions.grid.remove", Run = InteractionsGridCases.Remove });
            list.Add(new Case { Name = "interactions.grid.settings", Run = InteractionsGridCases.Settings });
            list.Add(new Case { Name = "interactions.grid.settings-locked", Run = InteractionsGridCases.SettingsLocked });
            list.Add(new Case { Name = "interactions.grid.reset", Run = InteractionsGridCases.Reset });
            list.Add(new Case { Name = "interactions.grid.settings-checked", Run = InteractionsGridCases.SettingsChecked });
            list.Add(new Case { Name = "interactions.gizmo.create", Run = GizmoCases.Create });
            list.Add(new Case { Name = "interactions.gizmo.reaction-owner", Run = GizmoCases.ReactionOwner });
            list.Add(new Case { Name = "interactions.gizmo.tick", Run = GizmoCases.Tick });
            list.Add(new Case { Name = "interactions.gizmo.tick-rounding", Run = GizmoCases.TickRounding });
            list.Add(new Case { Name = "interactions.gizmo.no-tick", Run = GizmoCases.NoTick });
            list.Add(new Case { Name = "interactions.gizmo.move", Run = GizmoCases.Move });
            list.Add(new Case { Name = "interactions.gizmo.ends-with-owner", Run = GizmoCases.EndsWithOwner });
            list.Add(new Case { Name = "interactions.gizmo.leave-in-grid", Run = GizmoCases.LeaveInGrid });
            list.Add(new Case { Name = "interactions.gizmo.cycle-stops", Run = GizmoCases.CycleStops });
            list.Add(new Case { Name = "interactions.gizmo.end-in-tick", Run = GizmoCases.EndInTick });
            list.Add(new Case { Name = "interactions.gizmo.made-in-tick", Run = GizmoCases.MadeInTick });
            list.Add(new Case { Name = "interactions.gizmo.long-tick", Run = GizmoCases.LongTick });
            list.Add(new Case { Name = "interactions.gizmo.cycle-lists", Run = GizmoCases.CycleLists });
            list.Add(new Case { Name = "interactions.gizmo.throwing-tick", Run = GizmoCases.ThrowingTick });
            list.Add(new Case { Name = "interactions.gizmo.tick-owner", Run = GizmoCases.TickOwner });
            list.Add(new Case { Name = "interactions.gizmo.throwing-reactions", Run = GizmoCases.ThrowingReactions });
            list.Add(new Case { Name = "interactions.gizmo.shadowed-fields", Run = GizmoCases.ShadowedFields });
            list.Add(new Case { Name = "interactions.tracking.only-when-asked", Run = TrackingCases.OnlyWhenAsked });
            list.Add(new Case { Name = "interactions.tracking.units-filled", Run = TrackingCases.UnitsFilled });
            list.Add(new Case { Name = "interactions.tracking.kinds-from-types", Run = TrackingCases.KindsFromTypes });
            list.Add(new Case { Name = "interactions.tracking.no-place-class", Run = TrackingCases.NoPlaceClass });
            list.Add(new Case { Name = "interactions.tracking.cell-check-spread", Run = TrackingCases.CellCheckSpread });
            list.Add(new Case { Name = "interactions.tracking.cell-check-counts", Run = TrackingCases.CellCheckCounts });
            list.Add(new Case { Name = "interactions.tracking.cell-check-after-ending", Run = TrackingCases.CellCheckAfterEnding });
            list.Add(new Case { Name = "interactions.tracking.no-place", Run = TrackingCases.NoPlace });
            list.Add(new Case { Name = "interactions.tracking.revived", Run = TrackingCases.Revived });
            list.Add(new Case { Name = "interactions.tracking.items-filled", Run = TrackingCases.ItemsFilled });
            list.Add(new Case { Name = "interactions.tracking.destructables-filled", Run = TrackingCases.DestructablesFilled });
            list.Add(new Case { Name = "interactions.tracking.ending", Run = TrackingCases.Ending });
            list.Add(new Case { Name = "interactions.tracking.ending-unseen", Run = TrackingCases.EndingUnseen });
            list.Add(new Case { Name = "interactions.tracking.new-ground-item", Run = TrackingCases.NewGroundItem });
            list.Add(new Case { Name = "interactions.tracking.read-once", Run = TrackingCases.ReadOnce });
            list.Add(new Case { Name = "interactions.tracking.read-outside-step", Run = TrackingCases.ReadOutsideStep });
            list.Add(new Case { Name = "interactions.tracking.gizmo-place", Run = TrackingCases.GizmoPlace });
            list.Add(new Case { Name = "interactions.tracking.first-use-items", Run = TrackingCases.FirstUseItems });
            list.Add(new Case { Name = "interactions.tracking.first-use-units", Run = TrackingCases.FirstUseUnits });
            list.Add(new Case { Name = "interactions.tracking.install-again", Run = TrackingCases.InstallAgain });
            list.Add(new Case { Name = "interactions.tracking.unit-read", Run = TrackingCases.UnitRead });
            list.Add(new Case { Name = "interactions.tracking.cell-check-fails", Run = TrackingCases.CellCheckFails });
            list.Add(new Case { Name = "interactions.find.in-range", Run = FindCases.InRange });
            list.Add(new Case { Name = "interactions.find.kinds", Run = FindCases.Kinds });
            list.Add(new Case { Name = "interactions.find.no-place-class", Run = FindCases.NoPlaceClass });
            list.Add(new Case { Name = "interactions.find.filter", Run = FindCases.Filter });
            list.Add(new Case { Name = "interactions.find.fills", Run = FindCases.Fills });
            list.Add(new Case { Name = "interactions.find.in-rect", Run = FindCases.InRect });
            list.Add(new Case { Name = "interactions.find.in-polygon", Run = FindCases.InPolygon });
            list.Add(new Case { Name = "interactions.find.along-line", Run = FindCases.AlongLine });
            list.Add(new Case { Name = "interactions.find.along-slanted-line", Run = FindCases.AlongSlantedLine });
            list.Add(new Case { Name = "interactions.find.closest", Run = FindCases.Closest });
            list.Add(new Case { Name = "interactions.find.closest-n", Run = FindCases.ClosestN });
            list.Add(new Case { Name = "interactions.find.stale", Run = FindCases.Stale });
            list.Add(new Case { Name = "interactions.find.no-place", Run = FindCases.NoPlace });
            list.Add(new Case { Name = "interactions.find.order", Run = FindCases.Order });
            list.Add(new Case { Name = "interactions.find.inside-reaction", Run = FindCases.InsideReaction });
            list.Add(new Case { Name = "interactions.find.nested", Run = FindCases.Nested });
            list.Add(new Case { Name = "interactions.find.throwing", Run = FindCases.Throwing });
            list.Add(new Case { Name = "interactions.find.throwing-closest", Run = FindCases.ThrowingClosest });
            list.Add(new Case { Name = "interactions.find.edge", Run = FindCases.Edge });
            list.Add(new Case { Name = "interactions.find.local-tracked", Run = FindCases.LocalTracked });
            list.Add(new Case { Name = "interactions.find.local-cached", Run = FindCases.LocalCached });
            list.Add(new Case { Name = "interactions.find.local-untracked", Run = FindCases.LocalUntracked });
            list.Add(new Case { Name = "interactions.find.local-mixed", Run = FindCases.LocalMixed });
            list.Add(new Case { Name = "interactions.interaction.meet-tick-part", Run = InteractionCases.MeetTickPart });
            list.Add(new Case { Name = "interactions.interaction.exact-range", Run = InteractionCases.ExactRange });
            list.Add(new Case { Name = "interactions.interaction.accepts", Run = InteractionCases.Accepts });
            list.Add(new Case { Name = "interactions.interaction.not-holder", Run = InteractionCases.NotHolder });
            list.Add(new Case { Name = "interactions.interaction.two-ways", Run = InteractionCases.TwoWays });
            list.Add(new Case { Name = "interactions.interaction.owned-by-buff", Run = InteractionCases.OwnedByBuff });
            list.Add(new Case { Name = "interactions.interaction.holder-ends", Run = InteractionCases.HolderEnds });
            list.Add(new Case { Name = "interactions.interaction.other-ends", Run = InteractionCases.OtherEnds });
            list.Add(new Case { Name = "interactions.interaction.holder-no-place", Run = InteractionCases.HolderNoPlace });
            list.Add(new Case { Name = "interactions.interaction.ends-in-meet", Run = InteractionCases.EndsInMeet });
            list.Add(new Case { Name = "interactions.interaction.ends-in-tick", Run = InteractionCases.EndsInTick });
            list.Add(new Case { Name = "interactions.interaction.other-ends-in-tick", Run = InteractionCases.OtherEndsInTick });
            list.Add(new Case { Name = "interactions.interaction.interval", Run = InteractionCases.Interval });
            list.Add(new Case { Name = "interactions.interaction.paused-interaction", Run = InteractionCases.PausedInteraction });
            list.Add(new Case { Name = "interactions.interaction.error-reported", Run = InteractionCases.ErrorReported });
            list.Add(new Case { Name = "interactions.interaction.made-in-reaction", Run = InteractionCases.MadeInReaction });
            list.Add(new Case { Name = "interactions.interaction.pairs-view", Run = InteractionCases.PairsView });
            list.Add(new Case { Name = "interactions.interaction.interact-checks", Run = InteractionCases.InteractChecks });
            list.Add(new Case { Name = "interactions.interaction.interact-in-created", Run = InteractionCases.InteractInCreated });
            list.Add(new Case { Name = "interactions.interaction.item-holder", Run = InteractionCases.ItemHolder });
            list.Add(new Case { Name = "interactions.interaction.destructable-holder", Run = InteractionCases.DestructableHolder });
            list.Add(new Case { Name = "interactions.interaction.pause-in-reaction", Run = InteractionCases.PauseInReaction });
            list.Add(new Case { Name = "interactions.interaction.shadowed-fields", Run = InteractionCases.ShadowedFields });
            list.Add(new Case { Name = "interactions.interaction.inner-loops", Run = InteractionCases.InnerLoops });
            list.Add(new Case { Name = "interactions.interaction.guarded-reactions", Run = InteractionCases.GuardedReactions });
            list.Add(new Case { Name = "interactions.interaction.check-fails", Run = InteractionCases.CheckFails });
            list.Add(new Case { Name = "interactions.interaction.other-kinds", Run = InteractionCases.OtherKinds });
            list.Add(new Case { Name = "interactions.interaction.spread-first-checks", Run = InteractionCases.SpreadFirstChecks });
            list.Add(new Case { Name = "interactions.interaction.corner-cells", Run = InteractionCases.CornerCells });
            list.Add(new Case { Name = "interactions.guard.turn", Run = GuardCases.Turn });
            list.Add(new Case { Name = "interactions.pair.wait", Run = PairCases.Wait });
            list.Add(new Case { Name = "interactions.pair.try-cooldown", Run = PairCases.TryCooldown });
            list.Add(new Case { Name = "interactions.pair.pause-resume", Run = PairCases.PauseResume });
            list.Add(new Case { Name = "interactions.pair.stop", Run = PairCases.Stop });
            list.Add(new Case { Name = "interactions.pair.data-and-since", Run = PairCases.DataAndSince });
            list.Add(new Case { Name = "interactions.pair.pair-owns", Run = PairCases.PairOwns });
            list.Add(new Case { Name = "interactions.tree", Run = TreeCases.Tree });
        }

        /// <summary>Adds the areas package's cases.</summary>
        private static void AddAreas(List<Case> list)
        {
            list.Add(new Case { Name = "areas.shapes.named", Run = AreaShapeCases.Named });
            list.Add(new Case { Name = "areas.shapes.unknown-name", Run = AreaShapeCases.UnknownName });
            list.Add(new Case { Name = "areas.shapes.circle", Run = AreaShapeCases.Circle });
            list.Add(new Case { Name = "areas.shapes.polygon", Run = AreaShapeCases.Polygon });
            list.Add(new Case { Name = "areas.shapes.of", Run = AreaShapeCases.Of });
            list.Add(new Case { Name = "areas.shapes.not-installed", Run = AreaShapeCases.NotInstalled });
            list.Add(new Case { Name = "areas.shapes.wrong-length", Run = AreaShapeCases.WrongLength });
            list.Add(new Case { Name = "areas.area.enter-stay-leave", Run = AreaCases.EnterStayLeave });
            list.Add(new Case { Name = "areas.area.accepts", Run = AreaCases.Accepts });
            list.Add(new Case { Name = "areas.area.already-inside", Run = AreaCases.AlreadyInside });
            list.Add(new Case { Name = "areas.area.sweep-and-event", Run = AreaCases.SweepAndEvent });
            list.Add(new Case { Name = "areas.area.death", Run = AreaCases.Death });
            list.Add(new Case { Name = "areas.area.removed", Run = AreaCases.Removed });
            list.Add(new Case { Name = "areas.area.area-ends", Run = AreaCases.AreaEnds });
            list.Add(new Case { Name = "areas.area.end-inside-enter", Run = AreaCases.EndInsideEnter });
            list.Add(new Case { Name = "areas.area.remove-inside-left", Run = AreaCases.RemoveInsideLeft });
            list.Add(new Case { Name = "areas.area.throwing", Run = AreaCases.Throwing });
            list.Add(new Case { Name = "areas.area.reaction-owner", Run = AreaCases.ReactionOwner });
            list.Add(new Case { Name = "areas.area.cells", Run = AreaCases.Cells });
            list.Add(new Case { Name = "areas.area.no-shape", Run = AreaCases.NoShape });
            list.Add(new Case { Name = "areas.area.region-before-install", Run = AreaCases.RegionBeforeInstall });
            list.Add(new Case { Name = "areas.area.fixed-once-added", Run = AreaCases.FixedOnceAdded });
            list.Add(new Case { Name = "areas.area.reads", Run = AreaCases.Reads });
            list.Add(new Case { Name = "areas.area.shared-timer", Run = AreaCases.SharedTimer });
            list.Add(new Case { Name = "areas.area.counts-from-first-unit", Run = AreaCases.CountsFromFirstUnit });
            list.Add(new Case { Name = "areas.area.hidden", Run = AreaCases.Hidden });
            list.Add(new Case { Name = "areas.area.hidden-then-gone", Run = AreaCases.HiddenThenGone });
            list.Add(new Case { Name = "areas.area.hidden-at-start", Run = AreaCases.HiddenAtStart });
            list.Add(new Case { Name = "areas.area.resurrected-reenters", Run = AreaCases.ResurrectedReenters });
            list.Add(new Case { Name = "areas.area.revive-not-twice", Run = AreaCases.ReviveNotTwice });
            list.Add(new Case { Name = "areas.area.corpse-removed", Run = AreaCases.CorpseRemoved });
            list.Add(new Case { Name = "areas.area.step-ends-both-areas", Run = AreaCases.StepEndsBothAreas });
            list.Add(new Case { Name = "areas.area.step-hands-over-and-restarts", Run = AreaCases.StepHandsOverAndRestarts });
            list.Add(new Case { Name = "areas.area.step-hands-over-and-keeps-running", Run = AreaCases.StepHandsOverAndKeepsRunning });
            list.Add(new Case { Name = "areas.area.step-leaves-and-reenters", Run = AreaCases.StepLeavesAndReenters });
        }

        /// <summary>Adds the input package's cases.</summary>
        private static void AddInput(List<Case> list)
        {
            list.Add(new Case { Name = "input.mouse.moved", Run = MouseCases.Moved });
            list.Add(new Case { Name = "input.mouse.not-a-place", Run = MouseCases.NotAPlace });
            list.Add(new Case { Name = "input.mouse.corner-of-the-world", Run = MouseCases.CornerOfTheWorld });
            list.Add(new Case { Name = "input.mouse.first-not-a-place", Run = MouseCases.FirstNotAPlace });
            list.Add(new Case { Name = "input.mouse.buttons", Run = MouseCases.Buttons });
            list.Add(new Case { Name = "input.mouse.unknown-button", Run = MouseCases.UnknownButton });
            list.Add(new Case { Name = "input.mouse.order", Run = MouseCases.Order });
            list.Add(new Case { Name = "input.mouse.only-written", Run = MouseCases.OnlyWritten });
            list.Add(new Case { Name = "input.mouse.throwing", Run = MouseCases.Throwing });
            list.Add(new Case { Name = "input.mouse.computer-and-left", Run = MouseCases.ComputerAndLeft });
            list.Add(new Case { Name = "input.mouse.no-mouse", Run = MouseCases.NoMouse });
            list.Add(new Case { Name = "input.mouse.per-slot", Run = MouseCases.PerSlot });
            list.Add(new Case { Name = "input.mouse.local", Run = MouseCases.Local });
            list.Add(new Case { Name = "input.mouse.local-unknown-focus", Run = MouseCases.LocalUnknownFocus });
            list.Add(new Case { Name = "input.mouse.setters", Run = MouseCases.Setters });
            list.Add(new Case { Name = "input.camera.reads", Run = CameraCases.Reads });
            list.Add(new Case { Name = "input.camera.changes", Run = CameraCases.Changes });
            list.Add(new Case { Name = "input.camera.named", Run = CameraCases.Named });
            list.Add(new Case { Name = "input.camera.unknown-camera", Run = CameraCases.UnknownCamera });
            list.Add(new Case { Name = "input.camera.wrong-data", Run = CameraCases.WrongData });
            list.Add(new Case { Name = "input.camera.not-installed", Run = CameraCases.NotInstalled });
        }

        /// <summary>Adds the terrain package's cases.</summary>
        private static void AddTerrain(List<Case> list)
        {
            list.Add(new Case { Name = "terrain.grid.pinned", Run = GridCases.Pinned });
            list.Add(new Case { Name = "terrain.grid.clamped", Run = GridCases.Clamped });
            list.Add(new Case { Name = "terrain.grid.wrong-length", Run = GridCases.WrongLength });
            list.Add(new Case { Name = "terrain.grid.not-installed", Run = GridCases.NotInstalled });
            list.Add(new Case { Name = "terrain.grid.playable", Run = GridCases.Playable });
            list.Add(new Case { Name = "terrain.grid.blocks", Run = GridCases.Blocks });
            list.Add(new Case { Name = "terrain.grid.many-blocks", Run = GridCases.ManyBlocks });
            list.Add(new Case { Name = "terrain.grid.alphabet", Run = GridCases.Alphabet });
            list.Add(new Case { Name = "terrain.height.corners", Run = HeightCases.Corners });
            list.Add(new Case { Name = "terrain.height.blend", Run = HeightCases.Blend });
            list.Add(new Case { Name = "terrain.height.clamped", Run = HeightCases.Clamped });
            list.Add(new Case { Name = "terrain.height.step", Run = HeightCases.Step });
            list.Add(new Case { Name = "terrain.height.cliff-blend", Run = HeightCases.CliffBlend });
            list.Add(new Case { Name = "terrain.height.ramp", Run = HeightCases.Ramp });
            list.Add(new Case { Name = "terrain.height.normal-and-slope", Run = HeightCases.NormalAndSlope });
            list.Add(new Case { Name = "terrain.height.not-a-number", Run = HeightCases.NotANumber });
            list.Add(new Case { Name = "terrain.height.not-installed", Run = HeightCases.NotInstalled });
            list.Add(new Case { Name = "terrain.pathing.flags", Run = PathingCases.Flags });
            list.Add(new Case { Name = "terrain.pathing.filled-once", Run = PathingCases.FilledOnce });
            list.Add(new Case { Name = "terrain.pathing.partial-block", Run = PathingCases.PartialBlock });
            list.Add(new Case { Name = "terrain.pathing.refresh", Run = PathingCases.Refresh });
            list.Add(new Case { Name = "terrain.pathing.refresh-span", Run = PathingCases.RefreshSpan });
            list.Add(new Case { Name = "terrain.pathing.outside", Run = PathingCases.Outside });
            list.Add(new Case { Name = "terrain.pathing.not-a-number", Run = PathingCases.NotANumber });
            list.Add(new Case { Name = "terrain.pathing.can-stand", Run = PathingCases.CanStand });
            list.Add(new Case { Name = "terrain.pathing.can-stand-water", Run = PathingCases.CanStandWater });
            list.Add(new Case { Name = "terrain.pathing.local-height", Run = PathingCases.LocalHeight });
            list.Add(new Case { Name = "terrain.pathing.not-installed", Run = PathingCases.NotInstalled });
            list.Add(new Case { Name = "terrain.pathing.reinstall", Run = PathingCases.Reinstall });
            list.Add(new Case { Name = "terrain.raycast.hill-hit", Run = GroundRayCases.HillHit });
            list.Add(new Case { Name = "terrain.raycast.miss", Run = GroundRayCases.Miss });
            list.Add(new Case { Name = "terrain.raycast.cliff", Run = GroundRayCases.Cliff });
            list.Add(new Case { Name = "terrain.raycast.starts-under", Run = GroundRayCases.StartsUnder });
            list.Add(new Case { Name = "terrain.raycast.zero-length", Run = GroundRayCases.ZeroLength });
            list.Add(new Case { Name = "terrain.raycast.vertical", Run = GroundRayCases.Vertical });
            list.Add(new Case { Name = "terrain.raycast.leaves-map", Run = GroundRayCases.LeavesMap });
            list.Add(new Case { Name = "terrain.raycast.long-ray", Run = GroundRayCases.LongRay });
            list.Add(new Case { Name = "terrain.raycast.not-finite", Run = GroundRayCases.NotFinite });
            list.Add(new Case { Name = "terrain.raycast.hit-values", Run = GroundRayCases.HitValues });
            list.Add(new Case { Name = "terrain.raycast.not-installed", Run = GroundRayCases.NotInstalled });
            list.Add(new Case { Name = "terrain.raycast.miss-makes-nothing", Run = MissCases.GroundMissMakesNothing });
        }

        /// <summary>Adds the maths package's cases.</summary>
        private static void AddMaths(List<Case> list)
        {
            list.Add(new Case { Name = "maths.scalars.lerp", Run = ScalarCases.Lerp });
            list.Add(new Case { Name = "maths.scalars.inverse-lerp", Run = ScalarCases.InverseLerp });
            list.Add(new Case { Name = "maths.scalars.remap", Run = ScalarCases.Remap });
            list.Add(new Case { Name = "maths.scalars.clamp-and-wrap", Run = ScalarCases.ClampAndWrap });
            list.Add(new Case { Name = "maths.scalars.move-toward", Run = ScalarCases.MoveToward });
            list.Add(new Case { Name = "maths.scalars.sign-and-near", Run = ScalarCases.SignAndNear });
            list.Add(new Case { Name = "maths.scalars.distance-and-angle", Run = ScalarCases.DistanceAndAngle });
            list.Add(new Case { Name = "maths.scalars.polar", Run = ScalarCases.Polar });
            list.Add(new Case { Name = "maths.scalars.angles-normalize", Run = ScalarCases.AnglesNormalize });
            list.Add(new Case { Name = "maths.scalars.angles-difference", Run = ScalarCases.AnglesDifference });
            list.Add(new Case { Name = "maths.scalars.angles-lerp-and-move", Run = ScalarCases.AnglesLerpAndMove });
            list.Add(new Case { Name = "maths.scalars.angles-trig", Run = ScalarCases.AnglesTrig });
            list.Add(new Case { Name = "maths.vector2.basics", Run = Vector2Cases.Basics });
            list.Add(new Case { Name = "maths.vector2.angles", Run = Vector2Cases.Angles });
            list.Add(new Case { Name = "maths.vector2.offset-and-rotate", Run = Vector2Cases.OffsetAndRotate });
            list.Add(new Case { Name = "maths.vector2.products", Run = Vector2Cases.Products });
            list.Add(new Case { Name = "maths.vector2.bounce-reflect-slide", Run = Vector2Cases.BounceReflectSlide });
            list.Add(new Case { Name = "maths.vector2.limits", Run = Vector2Cases.Limits });
            list.Add(new Case { Name = "maths.vector2.lerp", Run = Vector2Cases.Lerp });
            list.Add(new Case { Name = "maths.vector2.operators", Run = Vector2Cases.Operators });
            list.Add(new Case { Name = "maths.vector2.equality", Run = Vector2Cases.Equality });
            list.Add(new Case { Name = "maths.vector2.text", Run = Vector2Cases.Text });
            list.Add(new Case { Name = "maths.vector3.basics", Run = Vector3Cases.Basics });
            list.Add(new Case { Name = "maths.vector3.yaw-pitch", Run = Vector3Cases.YawPitch });
            list.Add(new Case { Name = "maths.vector3.cross", Run = Vector3Cases.Cross });
            list.Add(new Case { Name = "maths.vector3.bounce", Run = Vector3Cases.Bounce });
            list.Add(new Case { Name = "maths.vector3.from-vector2", Run = Vector3Cases.FromVector2 });
            list.Add(new Case { Name = "maths.vector3.equality-and-text", Run = Vector3Cases.EqualityAndText });
            list.Add(new Case { Name = "maths.ease.ends", Run = Vector3Cases.EaseEnds });
            list.Add(new Case { Name = "maths.ease.middles", Run = Vector3Cases.EaseMiddles });
            list.Add(new Case { Name = "maths.ease.clamped-and-overshoot", Run = Vector3Cases.EaseClampedAndOvershoot });
            list.Add(new Case { Name = "maths.shapes.bounds-corners", Run = ShapeCases.BoundsCorners });
            list.Add(new Case { Name = "maths.shapes.bounds-contains", Run = ShapeCases.BoundsContains });
            list.Add(new Case { Name = "maths.shapes.bounds-intersects", Run = ShapeCases.BoundsIntersects });
            list.Add(new Case { Name = "maths.shapes.bounds-clamp-expand", Run = ShapeCases.BoundsClampExpand });
            list.Add(new Case { Name = "maths.shapes.polygon-contains", Run = ShapeCases.PolygonContains });
            list.Add(new Case { Name = "maths.shapes.polygon-area-center", Run = ShapeCases.PolygonAreaCenter });
            list.Add(new Case { Name = "maths.shapes.polygon-copies", Run = ShapeCases.PolygonCopies });
            list.Add(new Case { Name = "maths.shapes.polygon-too-few", Run = ShapeCases.PolygonTooFew });
            list.Add(new Case { Name = "maths.raycast.circle-hit", Run = RaycastCases.CircleHit });
            list.Add(new Case { Name = "maths.raycast.circle-miss", Run = RaycastCases.CircleMiss });
            list.Add(new Case { Name = "maths.raycast.circle-short", Run = RaycastCases.CircleShort });
            list.Add(new Case { Name = "maths.raycast.circle-inside", Run = RaycastCases.CircleInside });
            list.Add(new Case { Name = "maths.raycast.circle-zero-length", Run = RaycastCases.CircleZeroLength });
            list.Add(new Case { Name = "maths.raycast.segment-hit", Run = RaycastCases.SegmentHit });
            list.Add(new Case { Name = "maths.raycast.segment-parallel", Run = RaycastCases.SegmentParallel });
            list.Add(new Case { Name = "maths.raycast.bounds-hit", Run = RaycastCases.BoundsHit });
            list.Add(new Case { Name = "maths.raycast.polygon-hit", Run = RaycastCases.PolygonHit });
            list.Add(new Case { Name = "maths.raycast.polygon-first-edge", Run = RaycastCases.PolygonFirstEdge });
            list.Add(new Case { Name = "maths.raycast.ray-at", Run = RaycastCases.RayAt });
            list.Add(new Case { Name = "maths.actors.position", Run = ActorMathsCases.Position });
            list.Add(new Case { Name = "maths.actors.place-and-move", Run = ActorMathsCases.PlaceAndMove });
            list.Add(new Case { Name = "maths.actors.distance-and-angle", Run = ActorMathsCases.DistanceAndAngle });
            list.Add(new Case { Name = "maths.actors.face", Run = ActorMathsCases.Face });
            list.Add(new Case { Name = "maths.actors.destructable", Run = ActorMathsCases.DestructableDistances });
            list.Add(new Case { Name = "maths.polygon.contains-floats", Run = ShapeCases.PolygonContainsFloats });
            list.Add(new Case { Name = "maths.lua.shared-not-copied", Run = LuaShapeCases.SharedNotCopied });
            list.Add(new Case { Name = "maths.lua.out-pair", Run = LuaShapeCases.OutPair });
            list.Add(new Case { Name = "maths.lua.number-types", Run = LuaShapeCases.NumberTypes });
            list.Add(new Case { Name = "maths.lua.miss-makes-nothing", Run = MissCases.MissMakesNothing });
            list.Add(new Case { Name = "maths.raycast.ray-values", Run = MissCases.RayValues });
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
            list.Add(new Case { Name = "hooks.taken-in-and-ending", Run = HookCases.TakenInAndEnding });
            list.Add(new Case { Name = "hooks.reset", Run = HookCases.WidgetHooksReset });
            list.Add(new Case { Name = "hooks.ending-two", Run = HookCases.EndingTwo });
            list.Add(new Case { Name = "hooks.ending-throwing", Run = HookCases.EndingThrowing });
            list.Add(new Case { Name = "hooks.ending-none", Run = HookCases.EndingNone });
            list.Add(new Case { Name = "hooks.ending-reset", Run = HookCases.EndingReset });
            list.Add(new Case { Name = "events.need-listens-once", Run = HookCases.NeedListensOnce });
            list.Add(new Case { Name = "actors.add-and-get", Run = ActorCases.AddAndGet });
            list.Add(new Case { Name = "actors.remove", Run = ActorCases.RemoveOne });
            list.Add(new Case { Name = "actors.remove-all", Run = ActorCases.RemoveAllOf });
            list.Add(new Case { Name = "actors.remove-of-class", Run = ActorCases.RemoveOf });
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
            list.Add(new Case { Name = "units.kill-twice", Run = UnitCases.KillTwice });
            list.Add(new Case { Name = "units.kill-leaves-no-life", Run = UnitCases.KillLeavesNoLife });
            list.Add(new Case { Name = "units.life-below-threshold", Run = UnitCases.LifeBelowThreshold });
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
            list.Add(new Case { Name = "units.name", Run = UnitCases.NameReadAndSet });
            list.Add(new Case { Name = "units.level-set-with-handle", Run = UnitCases.LevelSetWithHandleDoesNothing });
            list.Add(new Case { Name = "units.shared-without-handle", Run = UnitCases.SharedWithoutHandle });
            list.Add(new Case { Name = "defaults.reads-type-values", Run = UnitCases.DefaultReadsTypeValues });
            list.Add(new Case { Name = "defaults.once-per-type", Run = UnitCases.DefaultOncePerType });
            list.Add(new Case { Name = "defaults.removed-unit", Run = UnitCases.DefaultOfRemovedUnit });
            list.Add(new Case { Name = "defaults.live-actions-do-nothing", Run = UnitCases.DefaultLiveActionsDoNothing });
            list.Add(new Case { Name = "defaults.remove-does-nothing", Run = UnitCases.DefaultRemoveDoesNothing });
            list.Add(new Case { Name = "defaults.read-in-its-constructor", Run = UnitCases.DefaultReadInItsConstructor });
            list.Add(new Case { Name = "defaults.class-registered-later", Run = UnitCases.DefaultOfClassRegisteredLater });
            list.Add(new Case { Name = "defaults.class-without-its-maker-yet", Run = UnitCases.DefaultOfClassWithoutItsMakerYet });
            list.Add(new Case { Name = "defaults.unregistered-from-object", Run = UnitCases.DefaultOfUnregisteredFromObject });
            list.Add(new Case { Name = "defaults.unknown-type-name-empty", Run = UnitCases.DefaultNameOfUnknownTypeIsEmpty });
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
            list.Add(new Case { Name = "heroes.proper-name", Run = HeroCases.ProperName });
            list.Add(new Case { Name = "heroes.shared-without-handle", Run = HeroCases.SharedWithoutHandle });
            list.Add(new Case { Name = "items.registered-class", Run = ItemCases.RegisteredClass });
            list.Add(new Case { Name = "items.fallback-class", Run = ItemCases.FallbackClass });
            list.Add(new Case { Name = "items.pickup-order", Run = ItemCases.PickupOrder });
            list.Add(new Case { Name = "items.drop-order", Run = ItemCases.DropOrder });
            list.Add(new Case { Name = "items.listen", Run = ItemCases.Listens });
            list.Add(new Case { Name = "items.sweep", Run = ItemCases.SweepDetectsRemoval });
            list.Add(new Case { Name = "items.owned-create", Run = ItemCases.OwnedByOwner });
            list.Add(new Case { Name = "items.create-of-class", Run = ItemCases.CreateOfClass });
            list.Add(new Case { Name = "shadowed-fields.owner", Run = ShadowFieldCases.Owner });
            list.Add(new Case { Name = "shadowed-fields.hero", Run = ShadowFieldCases.Hero });
            list.Add(new Case { Name = "shadowed-fields.item", Run = ShadowFieldCases.Item });
            list.Add(new Case { Name = "damage.deal-from-code", Run = DamageCases.DealFromCode });
            list.Add(new Case { Name = "damage.deal-runs-reactions", Run = DamageCases.DealRunsReactions });
            list.Add(new Case { Name = "damage.deal-refused", Run = DamageCases.DealRefused });
            list.Add(new Case { Name = "damage.deal-is-attack", Run = DamageCases.DealIsAttack });
            list.Add(new Case { Name = "items.charges", Run = ItemCases.Charges });
            list.Add(new Case { Name = "items.player", Run = ItemCases.PlayerOf });
            list.Add(new Case { Name = "items.position", Run = ItemCases.Position });
            list.Add(new Case { Name = "items.move-keeps-the-place-the-game-gives", Run = ItemCases.MoveToKeepsThePlaceTheGameGives });
            list.Add(new Case { Name = "items.move-removed-keeps-the-last-place", Run = ItemCases.MoveToOnARemovedItemKeepsTheLastPlace });
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
            list.Add(new Case { Name = "items.shared-without-handle", Run = ItemCases.SharedWithoutHandle });
            list.Add(new Case { Name = "destructable.position", Run = DestructableCases.Position });
            list.Add(new Case { Name = "destructables.watch", Run = DestructableCases.Watch });
            list.Add(new Case { Name = "destructables.sweep", Run = DestructableCases.SweepDetectsRemoval });
            list.Add(new Case { Name = "destructables.remove-after-death", Run = DestructableCases.RemoveAfterDeath });
            list.Add(new Case { Name = "destructables.remove-living", Run = DestructableCases.RemoveLiving });
            list.Add(new Case { Name = "components.ended-in-onremoved", Run = EndedCases.ReadInOnRemoved });
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
            list.Add(new Case { Name = "damage-package.lethal-after-bonus", Run = LandingCases.LethalAfterBonus });
            list.Add(new Case { Name = "damage-package.killed-only-killing-hit", Run = LandingCases.KilledOnlyKillingHit });
            list.Add(new Case { Name = "damage-package.bonus-kill-credits-the-bonus", Run = LandingCases.BonusKillCreditsTheBonus });
            list.Add(new Case { Name = "damage-package.lethal-stops-when-spared", Run = LandingCases.LethalStopsWhenSpared });
            list.Add(new Case { Name = "damage-package.removed-target-not-killed", Run = LandingCases.RemovedTargetNotKilled });
            list.Add(new Case { Name = "damage-package.outer-found-below-stray", Run = LandingCases.OuterFoundBelowStray });
            list.Add(new Case { Name = "damage-package.deal-stopped-returns-null", Run = DealCases.DealStoppedReturnsNull });
            list.Add(new Case { Name = "damage-package.raw-deal-chain-cut", Run = DealCases.RawDealChainCut });
            list.Add(new Case { Name = "damage-package.code-deal-of-zero-runs", Run = DealCases.CodeDealOfZeroRuns });
            list.Add(new Case { Name = "damage-package.zero-hit-quiet", Run = StageCases.ZeroHitQuiet });
            list.Add(new Case { Name = "damage-package.sourceless-hit-runs", Run = StageCases.SourcelessHitRuns });
            list.Add(new Case { Name = "damage-package.late-reaction-runs", Run = StageCases.LateReactionRuns });
            list.Add(new Case { Name = "damage-package.zero-attack-runs", Run = StageCases.ZeroAttackRuns });
            list.Add(new Case { Name = "damage-package.lethal-reaction-kill-credits-its-hit", Run = LandingCases.LethalReactionKillCreditsItsHit });
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
            list.Add(new Case { Name = "floats.beyond-the-rewrite", Run = FloatCases.Beyond });
            list.Add(new Case { Name = "floats.ints-stay", Run = FloatCases.IntsStay });
            list.Add(new Case { Name = "floats.ints-start-as-integers", Run = FloatCases.IntsStart });
            list.Add(new Case { Name = "floats.made-floats-in-calls", Run = FloatCases.MadeFloatsInCalls });
            list.Add(new Case { Name = "text.nullable-concat", Run = TextCases.NullableConcat });
            list.Add(new Case { Name = "text.conditional-access-concat", Run = TextCases.ConditionalAccessConcat });
            list.Add(new Case { Name = "text.nullable-interpolation", Run = TextCases.NullableInterpolation });
            list.Add(new Case { Name = "text.nullable-operands", Run = TextCases.NullableOperands });
            list.Add(new Case { Name = "text.nullable-from-int", Run = TextCases.NullableFromInt });
            list.Add(new Case { Name = "text.nullable-char", Run = TextCases.NullableChar });
            list.Add(new Case { Name = "text.nullable-struct", Run = TextCases.NullableStruct });
            list.Add(new Case { Name = "text.nullable-to-string", Run = TextCases.NullableToString });
            list.Add(new Case { Name = "text.plain", Run = TextCases.Plain });
            list.Add(new Case { Name = "text.nullable-get-value-or-default", Run = TextCases.NullableGetValueOrDefault });
            list.Add(new Case { Name = "text.nullable-enum-to-string", Run = TextCases.NullableEnumToString });
            list.Add(new Case { Name = "collection-expressions.everywhere", Run = CollectionExpressionCases.Everywhere });
            list.Add(new Case { Name = "collection-expressions.spread", Run = CollectionExpressionCases.Spread });
            list.Add(new Case { Name = "templates.static-through-using-static", Run = TemplateCases.StaticThroughUsingStatic });
            list.Add(new Case { Name = "templates.instance-without-receiver", Run = TemplateCases.InstanceWithoutReceiver });
            list.Add(new Case { Name = "templates.instance-put", Run = TemplateCases.InstancePut });
            list.Add(new Case { Name = "strings.escapes", Run = StringCases.Escapes });
            list.Add(new Case { Name = "strings.raw-and-verbatim", Run = StringCases.RawAndVerbatim });
            list.Add(new Case { Name = "strings.utf8", Run = StringCases.Utf8 });
            list.Add(new Case { Name = "strings.interpolated", Run = StringCases.Interpolated });
            list.Add(new Case { Name = "strings.percent", Run = StringCases.Percent });
            list.Add(new Case { Name = "strings.folded", Run = StringCases.Folded });
            list.Add(new Case { Name = "conditionals.nullable-first-branch", Run = ConditionalCases.NullableFirstBranch });
            list.Add(new Case { Name = "conditionals.nullable-bool-first-branch", Run = ConditionalCases.NullableBoolFirstBranch });
            list.Add(new Case { Name = "conditionals.struct-type-parameter", Run = ConditionalCases.StructTypeParameter });
            list.Add(new Case { Name = "conditionals.still-right", Run = ConditionalCases.StillRight });
            list.Add(new Case { Name = "compiler.out-discard-lambda", Run = CompilerCases.OutDiscardLambda });
            list.Add(new Case { Name = "compiler.return-last-in-loop", Run = CompilerCases.ReturnLastInLoop });
            list.Add(new Case { Name = "compiler.continue-in-switch", Run = CompilerCases.ContinueInSwitch });
            list.Add(new Case { Name = "compiler.primary-constructors", Run = CompilerCases.PrimaryConstructors });
            list.Add(new Case { Name = "compiler.initializer-temporaries", Run = CompilerCases.InitializerTemporaries });
            list.Add(new Case { Name = "compiler.foreach-disposes", Run = CompilerCases.ForeachDisposes });
            list.Add(new Case { Name = "compiler.private-base-fields", Run = CompilerCases.PrivateBaseFields });
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
