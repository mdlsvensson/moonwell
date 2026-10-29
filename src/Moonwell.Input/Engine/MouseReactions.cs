using System;
using System.Collections.Generic;

namespace Moonwell.Engine
{
    /// <summary>
    /// Takes the game's mouse events into the player's Mouse and runs the map-wide reactions: the game's components in add
    /// order, then the player's own, never another player's. The watchers of the game and of each player are kept in a
    /// list, read again only when that actor's components change, so an event does not look at every component again (a
    /// map with many areas has many game components, and the moves come 120 to 240 times a second).
    /// </summary>
    internal static class MouseReactions
    {
        /// <summary>One actor's map components that write a mouse reaction, with their masks, as read from its list of components.</summary>
        private sealed class Watchers
        {
            /// <summary>The actor they were read from; null before the first read.</summary>
            internal Actor? Actor;

            /// <summary>The actor's list of components they were read from: core makes a new list when it drops a removed component.</summary>
            internal List<Component?>? Source;

            /// <summary>How many components that list held then: an added one makes it more.</summary>
            internal int Count;

            /// <summary>The watchers, in add order; a new list at each read, so a loop over the old one is not changed under it.</summary>
            internal List<MapComponent> Found = new List<MapComponent>();

            /// <summary>The mask of each watcher, in the same order.</summary>
            internal List<int> Masks = new List<int>();
        }

        /// <summary>The reaction methods, one bit each: a class's mask holds the ones it has.</summary>
        private const int Moved = 1, Pressed = 2, Released = 4;

        /// <summary>The mask of each class seen, keyed by the class.</summary>
        private static readonly Dictionary<object, int> masks = new Dictionary<object, int>();

        /// <summary>The game's watchers.</summary>
        private static Watchers gameWatchers = new Watchers();

        /// <summary>Each slot's player's watchers; null until that player's first event.</summary>
        private static Watchers?[] playerWatchers = new Watchers?[Player.MaxSlots];

        /// <summary>How many times a component's mask was looked up; for the cases.</summary>
        internal static int Lookups;

        /// <summary>Forgets every class's mask and the watchers read; Install calls it.</summary>
        internal static void Reset()
        {
            masks.Clear();
            gameWatchers = new Watchers();
            playerWatchers = new Watchers?[Player.MaxSlots];
        }

        /// <summary>The game's move event: takes the place, then runs OnMouseMoved.</summary>
        internal static void OnMoved()
        {
            var mouse = Event();
            if (mouse == null)
            {
                return;
            }

            var world = InputSystem.World;
            mouse.Take(world.EventMouseX(), world.EventMouseY(), InputSystem.CornerX, InputSystem.CornerY);
            Run(mouse, Moved, MouseButton.Left);
        }

        /// <summary>The game's press event: takes the place, marks the button down, then runs OnMousePressed.</summary>
        internal static void OnPressed()
        {
            var mouse = Event();
            if (mouse == null)
            {
                return;
            }

            var world = InputSystem.World;
            mouse.Take(world.EventMouseX(), world.EventMouseY(), InputSystem.CornerX, InputSystem.CornerY);
            var number = world.EventMouseButton();
            if (number < 1 || number > 3)
            {
                return;
            }

            var button = (MouseButton)number;
            mouse.SetDown(button, true);
            Run(mouse, Pressed, button);
        }

        /// <summary>The game's release event: takes the place, marks the button up, then runs OnMouseReleased.</summary>
        internal static void OnReleased()
        {
            var mouse = Event();
            if (mouse == null)
            {
                return;
            }

            var world = InputSystem.World;
            mouse.Take(world.EventMouseX(), world.EventMouseY(), InputSystem.CornerX, InputSystem.CornerY);
            var number = world.EventMouseButton();
            if (number < 1 || number > 3)
            {
                return;
            }

            var button = (MouseButton)number;
            mouse.SetDown(button, false);
            Run(mouse, Released, button);
        }

        /// <summary>The mouse of the player the event being delivered belongs to.</summary>
        private static Mouse? Event()
        {
            var player = Player.Of(InputSystem.World.EventPlayer());
            return player == null ? null : InputSystem.MouseOf(player.Slot);
        }

        /// <summary>Runs a reaction on the game's components, then on the mouse's player's.</summary>
        private static void Run(Mouse mouse, int bit, MouseButton button)
        {
            RunMap(Game.Instance, gameWatchers, mouse, bit, button);
            var player = Player.Existing(mouse.Slot);
            if (player != null)
            {
                var watchers = playerWatchers[mouse.Slot];
                if (watchers == null)
                {
                    watchers = new Watchers();
                    playerWatchers[mouse.Slot] = watchers;
                }

                RunMap(player, watchers, mouse, bit, button);
            }
        }

        /// <summary>Runs the reaction on an actor's watchers, in add order: those there when the event came (one added by a reaction runs from the next event), skipping one that has ended.</summary>
        private static void RunMap(Actor actor, Watchers watchers, Mouse mouse, int bit, MouseButton button)
        {
            if (watchers.Actor != actor || watchers.Source != actor.components || watchers.Count != actor.components.Count)
            {
                Read(watchers, actor);
            }

            var found = watchers.Found;
            var masksFound = watchers.Masks;
            for (var i = 0; i < found.Count; i++)
            {
                var component = found[i];
                if (!component.HasEnded && (masksFound[i] & bit) != 0)
                {
                    Call(component, mouse, bit, button);
                }
            }
        }

        /// <summary>Reads an actor's watchers from its list of components: the map components that have not ended and write a mouse reaction.</summary>
        private static void Read(Watchers watchers, Actor actor)
        {
            var components = actor.components;
            var found = new List<MapComponent>();
            var masksFound = new List<int>();
            for (var i = 0; i < components.Count; i++)
            {
                var component = components[i] as MapComponent;
                if (component != null && !component.HasEnded)
                {
                    var mask = MaskOf(component);
                    if (mask != 0)
                    {
                        found.Add(component);
                        masksFound.Add(mask);
                    }
                }
            }

            watchers.Actor = actor;
            watchers.Source = components;
            watchers.Count = components.Count;
            watchers.Found = found;
            watchers.Masks = masksFound;
        }

        /// <summary>Calls one reaction; a failing reaction is reported.</summary>
        private static void Call(Scope obj, Mouse mouse, int bit, MouseButton button)
        {
            try
            {
                switch (bit)
                {
                    case Moved: ((IMouseWatch)obj).OnMouseMoved(mouse); break;
                    case Pressed: ((IMouseWatch)obj).OnMousePressed(mouse, button); break;
                    default: ((IMouseWatch)obj).OnMouseReleased(mouse, button); break;
                }
            }
            catch (Exception error)
            {
                Errors.Report(Names.NameOf(obj) + "." + MethodName(bit), error);
            }
        }

        /// <summary>The bits of the reaction methods an object's class writes, worked out once per class. Only methods the class writes count: in Lua an interface's empty defaults are not there.</summary>
        private static int MaskOf(object obj)
        {
            Lookups++;
            var key = Names.ClassOf(obj);
            if (masks.TryGetValue(key, out var mask))
            {
                return mask;
            }

            mask = 0;
            if (obj is IMouseWatch)
            {
                mask = Has(obj, "OnMouseMoved", Moved) | Has(obj, "OnMousePressed", Pressed) | Has(obj, "OnMouseReleased", Released);
            }

            masks[key] = mask;
            return mask;
        }

        /// <summary>The bit when the object's class writes the method, else 0.</summary>
        private static int Has(object obj, string name, int bit)
        {
            return Hooks.Overrides(obj, name, typeof(object)) ? bit : 0;
        }

        /// <summary>The method name of a bit, for an error line.</summary>
        private static string MethodName(int bit)
        {
            return bit == Moved ? "OnMouseMoved" : bit == Pressed ? "OnMousePressed" : "OnMouseReleased";
        }
    }
}
