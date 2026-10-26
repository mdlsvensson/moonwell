using System;
using System.Collections.Generic;

namespace Moonwell.Engine
{
    /// <summary>Takes the game's mouse events into the player's Mouse and runs the map-wide reactions: the game's components in add order, then the player's own, never another player's.</summary>
    internal static class MouseReactions
    {
        /// <summary>The reaction methods, one bit each: a class's mask holds the ones it has.</summary>
        private const int Moved = 1, Pressed = 2, Released = 4;

        /// <summary>The mask of each class seen, keyed by the class.</summary>
        private static readonly Dictionary<object, int> masks = new Dictionary<object, int>();

        /// <summary>Forgets every class's mask; Install calls it.</summary>
        internal static void Reset()
        {
            masks.Clear();
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
            RunMap(Game.Instance, mouse, bit, button);
            var player = Player.Existing(mouse.Slot);
            if (player != null)
            {
                RunMap(player, mouse, bit, button);
            }
        }

        /// <summary>Runs the reaction on an actor's map components, in add order.</summary>
        private static void RunMap(Actor actor, Mouse mouse, int bit, MouseButton button)
        {
            var count = actor.BeginDispatch();
            for (var i = 0; i < count; i++)
            {
                var component = actor.ComponentAt(i) as MapComponent;
                if (component != null && !component.HasEnded && (MaskOf(component) & bit) != 0)
                {
                    Call(component, mouse, bit, button);
                }
            }

            actor.EndDispatch();
        }

        /// <summary>Calls one reaction with the component as the current owner; a failing reaction is reported.</summary>
        private static void Call(Owner obj, Mouse mouse, int bit, MouseButton button)
        {
            var previous = Owner.current;
            Owner.current = obj;
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

            Owner.current = previous;
        }

        /// <summary>The bits of the reaction methods an object's class writes, worked out once per class. Only methods the class writes count: in Lua an interface's empty defaults are not there.</summary>
        private static int MaskOf(object obj)
        {
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
