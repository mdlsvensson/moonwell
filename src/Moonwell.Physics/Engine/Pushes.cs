using System;
using System.Collections.Generic;

namespace Moonwell.Engine
{
    /// <summary>
    /// The pushed units: each step of the physics pass, before the missiles, each is moved along its push with SetUnitX/Y
    /// (which keeps its order) and put in its cell at once, kept on ground it can stand on, slowed, and lifted or brought
    /// down. The point its collision size ahead is checked against the map's walls (the line from the unit to it crossing
    /// one it is not lifted over), with the terrain's pathing (flyable for a flyer) and then, for a unit that does not
    /// fly, the item trick, which also sees trees and buildings; on a bridge's deck, the deck's area instead, its edge a
    /// rail; blocked, it slides along the edge
    /// on the free axis, losing the speed into it, or stops, and OnSlammed runs. The reactions run on the unit's class,
    /// then its components and buffs in the order they were added, each guarded, with the unit as the current owner; a
    /// method a class does not write is not called (in Lua an interface's empty defaults are not there).
    /// </summary>
    internal static class Pushes
    {
        /// <summary>The bit of OnPushed in a class's mask.</summary>
        private const int PushedBit = 1;

        /// <summary>The bit of OnSlammed in a class's mask.</summary>
        private const int SlammedBit = 2;

        /// <summary>The bit of OnGrounded in a class's mask.</summary>
        private const int GroundedBit = 4;

        /// <summary>Degrees to radians.</summary>
        private const float Radians = 0.0174532925f;

        /// <summary>The cosine of an eighth of a turn, for the points of a unit's edge between the axes.</summary>
        private const float Slant = 0.70710678f;

        /// <summary>The most the ground beside a bridge's deck may differ from it for a pushed unit to cross between them; more, and the deck's edge is a rail.</summary>
        private const float DeckStep = 64.0f;

        /// <summary>The pushes, in the order they began; an ended one stays until the step packs the list. Made on first use after an install. It holds no null, so the pass reads it raw.</summary>
        private static List<Pushed>? pushes;

        /// <summary>How many pushes have not ended.</summary>
        private static int active;

        /// <summary>Whether the package has raised the interactions cycle's Awake count, which it does while any unit is pushed.</summary>
        private static bool raised;

        /// <summary>Whether a push has ended since the list was last packed.</summary>
        private static bool dropped;

        /// <summary>Whether the step is moving the pushes, so the list is not emptied under it.</summary>
        private static bool stepping;

        /// <summary>Whether a reaction has run since the step last looked: one may have killed, hidden or removed the unit.</summary>
        private static bool reacted;

        /// <summary>The reactions each class writes, keyed by the class (never by an actor); made on first use.</summary>
        private static Dictionary<object, int>? masks;

        /// <summary>Forgets every push. When the interactions package was not installed afresh since, its Awake count is lowered for the pushes that held it; an install of it has cleared the count already.</summary>
        internal static void Reset(bool trackingKept)
        {
            if (raised && trackingKept)
            {
                Tracking.Awake = Tracking.Awake - 1;
            }

            pushes = null;
            active = 0;
            raised = false;
            dropped = false;
            stepping = false;
            masks = null;
        }

#if DEBUG
        /// <summary>How many units are being pushed; the debug mode's statistics read it.</summary>
        internal static int Active
        {
            get { return Physics.IsInstalled ? active : 0; }
        }

#endif
        /// <summary>The push of a unit that has not ended; null when the unit is not pushed, or the package is not installed.</summary>
        internal static Pushed? Find(Unit unit)
        {
            var list = pushes;
            if (list == null || active == 0 || !Physics.IsInstalled)
            {
                return null;
            }

            var count = list.Count;
            for (var i = 0; i < count; i++)
            {
                var push = Raw.At(list, i);
                if (!push.ended && object.ReferenceEquals(push.unit, unit))
                {
                    return push;
                }
            }

            return null;
        }

        /// <summary>Adds a velocity to a unit's push and runs OnPushed with the pusher; nothing for no velocity, or for a unit that cannot be pushed. The pusher is kept for the slams after, unless a later push gives another.</summary>
        internal static void Push(Unit unit, float vx, float vy, float vz, Unit? pusher)
        {
            if (vx == 0.0f && vy == 0.0f && vz == 0.0f)
            {
                return;
            }

            var push = Begin(unit);
            if (push == null)
            {
                return;
            }

            push.vx += vx;
            push.vy += vy;
            if (vz != 0.0f)
            {
                Lift(push);
                push.vz += vz;
            }

            if (pusher != null)
            {
                push.pusher = pusher;
            }

            React(push, PushedBit, pusher, null, 0.0f);
        }

        /// <summary>Knocks a unit a distance along an angle in degrees over a time: adds 2 * distance / seconds along the angle, and the push slows by that speed / seconds a second for that time. A distance of 0 does nothing; a time of 0 or less fails in a development build and does nothing in a release build.</summary>
        internal static void Knockback(Unit unit, float angle, float distance, float seconds)
        {
            Dev.Check(seconds > 0.0f, "Unit.Knockback: seconds must be more than 0");
            if (!(seconds > 0.0f) || distance == 0.0f)
            {
                return;
            }

            var push = Begin(unit);
            if (push == null)
            {
                return;
            }

            var speed = 2.0f * distance / seconds;
            var radians = angle * Radians;
            push.vx += speed * Trig.Cos(radians);
            push.vy += speed * Trig.Sin(radians);
            push.knockSlowing = (speed < 0.0f ? -speed : speed) / seconds;
            push.knockLeft = seconds;
            React(push, PushedBit, null, null, 0.0f);
        }

        /// <summary>Gives a unit's push velocity; zero when it is not pushed, and a vertical velocity only while lifted.</summary>
        internal static void Velocity(Unit unit, out float vx, out float vy, out float vz)
        {
            var push = Find(unit);
            if (push == null)
            {
                vx = 0.0f;
                vy = 0.0f;
                vz = 0.0f;
                return;
            }

            vx = push.vx;
            vy = push.vy;
            vz = push.lifted ? push.vz : 0.0f;
        }

        /// <summary>Ends a unit's push with no reaction, putting a lifted unit back at its height; nothing when it is not pushed.</summary>
        internal static void Stop(Unit unit)
        {
            var push = Find(unit);
            if (push != null)
            {
                End(push, true);
            }
        }

        /// <summary>Moves each push that was in the list when the step began and has not ended, in the order they began; then packs away the ended ones. A push begun by a reaction moves from the next step. A push whose step throws loses that step alone: the error is reported and the rest move, and so do the step's missiles after.</summary>
        internal static void Run(float step)
        {
            var list = pushes;
            if (list == null)
            {
                return;
            }

            var count = list.Count;
            var at = 0;
            stepping = true;
            while (at < count)
            {
                // One try a step, not one a push: in Lua a try is closures. A throw goes on from the push after.
                try
                {
                    for (; at < count; at++)
                    {
                        var push = Raw.At(list, at);
                        if (!push.ended)
                        {
                            Step(push, step);
                        }
                    }
                }
                catch (Exception error)
                {
                    at++;
                    Errors.Report("Moonwell.Physics", error);
                }
            }

            stepping = false;

            if (dropped)
            {
                Pack(list);
            }
        }

        /// <summary>The push of a unit, begun with no velocity when it has none; null for a unit that cannot be pushed: one that has left the game, is dead, hidden or loaded, a structure, or one whose Body is immovable.</summary>
        private static Pushed? Begin(Unit unit)
        {
            if (unit.Handle == null || unit.IsGone || unit.ending || unit.HasEnded)
            {
                return null;
            }

            var world = Physics.World;
            var handle = unit.Handle;
            var body = unit.Get<Body>();
            if (body != null && body.HasEnded)
            {
                body = null;
            }

            if ((body != null && body.Immovable) || world.IsStructure(handle) || !Runtime.World.IsAlive(handle) || Interactions.World.IsHidden(handle) || Interactions.World.IsLoaded(handle))
            {
                return null;
            }

            var push = Find(unit);
            if (push != null)
            {
                push.body = body;
                return push;
            }

            push = new Pushed(unit, world.IsFlying(handle), world.CollisionSize(handle));
            push.body = body;
            if (pushes == null)
            {
                pushes = new List<Pushed>();
            }

            Raw.Append(pushes, push);
            active++;
            if (!raised)
            {
                raised = true;
                Tracking.Awake = Tracking.Awake + 1;
            }

            return push;
        }

        /// <summary>Lifts a push that is not lifted yet: its height is the unit's fly height, and it comes back to its type's fly height (the ground for most units that do not fly), so a hovering unit is not left on the ground.</summary>
        private static void Lift(Pushed push)
        {
            if (push.lifted)
            {
                return;
            }

            push.lifted = true;
            push.vz = 0.0f;
            push.height = push.unit.FlyHeight;
            push.home = Physics.World.DefaultFlyHeight(push.unit.Handle);
        }

        /// <summary>Ends a push; with settle, a lifted unit that is still in the game is put back at its height. When no push is left, the cycle may sleep.</summary>
        private static void End(Pushed push, bool settle)
        {
            if (push.ended)
            {
                return;
            }

            push.ended = true;
            dropped = true;
            active--;
            if (settle && push.lifted && !push.unit.IsGone && !push.unit.HasEnded)
            {
                push.unit.FlyHeight = push.home;
            }

            push.lifted = false;
            push.vx = 0.0f;
            push.vy = 0.0f;
            push.vz = 0.0f;
            if (active == 0)
            {
                // Out of a step the cycle may sleep before it packs the list again, so the ended pushes go now.
                if (!stepping && pushes != null)
                {
                    Raw.Truncate(pushes, 0);
                    dropped = false;
                }

                if (raised)
                {
                    raised = false;
                    Tracking.Awake = Tracking.Awake - 1;
                }
            }
        }

        /// <summary>A push's step: it ends with no reaction once the unit has left the game, died, been hidden or loaded; otherwise it moves along the ground, then up or down, and ends once it has no speed and is not lifted.</summary>
        private static void Step(Pushed push, float step)
        {
            var unit = push.unit;
            if (Gone(unit))
            {
                End(push, !unit.IsGone && !unit.HasEnded);
                return;
            }

            reacted = false;
            if (push.vx != 0.0f || push.vy != 0.0f)
            {
                Slide(push, step);
            }

            if (push.ended || Unplaced(push))
            {
                return;
            }

            if (push.lifted)
            {
                if (unit.HasEnded || unit.IsGone)
                {
                    End(push, false);
                    return;
                }

                if (push.flyer)
                {
                    Drift(push, step);
                }
                else
                {
                    Fall(push, step);
                }

                if (push.ended || Unplaced(push))
                {
                    return;
                }
            }

            if (!push.ended && push.vx == 0.0f && push.vy == 0.0f && !push.lifted)
            {
                End(push, false);
            }
        }

        /// <summary>After a reaction has run in the step: ends the push, with no more reactions, when the reaction killed, hid, loaded or removed the unit, and says so.</summary>
        private static bool Unplaced(Pushed push)
        {
            if (!reacted)
            {
                return false;
            }

            reacted = false;
            var unit = push.unit;
            if (!Gone(unit))
            {
                return false;
            }

            End(push, !unit.IsGone && !unit.HasEnded);
            return true;
        }

        /// <summary>Whether a unit has no place for a push: it has left the game, is leaving it, is dead, hidden or loaded.</summary>
        private static bool Gone(Unit unit)
        {
            if (unit.HasEnded || unit.ending || unit.IsGone)
            {
                return true;
            }

            var handle = unit.Handle;
            var host = Interactions.World;
            return !Runtime.World.IsAlive(handle) || host.IsHidden(handle) || host.IsLoaded(handle);
        }

        /// <summary>Moves a push along the ground for a step, slowing it evenly (the distance is exact for the slowing, so a knockback ends where it should); blocked, it slides along the edge or stops.</summary>
        private static void Slide(Pushed push, float step)
        {
            var vx = push.vx;
            var vy = push.vy;
            var speed = Trig.Sqrt(vx * vx + vy * vy);
            if (!(speed > 0.0f))
            {
                // Too slow to square: no direction to move in.
                push.vx = 0.0f;
                push.vy = 0.0f;
                return;
            }

            float slowing;
            if (push.knockLeft > 0.0f)
            {
                slowing = push.knockSlowing;
                push.knockLeft -= step;
            }
            else
            {
                slowing = Friction(push);
            }

            var after = speed - slowing * step;
            float distance;
            if (after <= 0.0f)
            {
                after = 0.0f;
                distance = speed * speed / (2.0f * slowing);
            }
            else
            {
                distance = (speed + after) * 0.5f * step;
            }

            var ux = vx / speed;
            var uy = vy / speed;
            var unit = push.unit;
            var x = unit.X;
            var y = unit.Y;
            var size = push.size;
            var toX = x + ux * distance;
            var toY = y + uy * distance;
            if (Free(push, x, y, toX, toY, toX + ux * size, toY + uy * size))
            {
                Place(unit, toX, toY);
                push.vx = ux * after;
                push.vy = uy * after;
                return;
            }

            // Blocked: slide on the free axis, the faster one first, losing the speed into the edge; else stop.
            var alongXFirst = (vx < 0.0f ? -vx : vx) >= (vy < 0.0f ? -vy : vy);
            for (var turn = 0; turn < 2; turn++)
            {
                var alongX = (turn == 0) == alongXFirst;
                if (vx == 0.0f || vy == 0.0f)
                {
                    break;
                }

                if (alongX)
                {
                    if (Free(push, x, y, toX, y, toX + (ux > 0.0f ? size : -size), y))
                    {
                        Place(unit, toX, y);
                        push.vx = ux * after;
                        push.vy = 0.0f;
                        Slammed(push, vy < 0.0f ? -vy : vy, toX, y, 0.0f, vy > 0.0f ? -1.0f : 1.0f);
                        return;
                    }
                }
                else if (Free(push, x, y, x, toY, x, toY + (uy > 0.0f ? size : -size)))
                {
                    Place(unit, x, toY);
                    push.vx = 0.0f;
                    push.vy = uy * after;
                    Slammed(push, vx < 0.0f ? -vx : vx, x, toY, vx > 0.0f ? -1.0f : 1.0f, 0.0f);
                    return;
                }
            }

            push.vx = 0.0f;
            push.vy = 0.0f;
            push.knockLeft = 0.0f;
            Slammed(push, speed, x, y, -ux, -uy);
        }

        /// <summary>How fast a push slows when no knockback's slowing holds: its unit's Body.Friction, else 600.</summary>
        private static float Friction(Pushed push)
        {
            var body = push.body;
            return body != null && !body.HasEnded ? body.Friction : Body.DefaultFriction;
        }

        /// <summary>
        /// Whether a pushed unit at (fromX, fromY) may move to (centreX, centreY), looking ahead to a point (x, y), its
        /// collision size ahead along the move: the point inside the world's bounds, with no wall it is not lifted over
        /// across the way there, and flyable for a flyer; for a unit that does not fly, walkable and where an item lands, or
        /// on a bridge's deck. A deck is ground inside its area, whatever the terrain's pathing reads under it, and its edge
        /// is a rail, except where the ground beyond is within DeckStep of it (its ends, on the banks). The rail stops the
        /// unit's body, not its middle: on a deck, its collision size around the place it moves to stays on the deck.
        /// </summary>
        private static bool Free(Pushed push, float fromX, float fromY, float centreX, float centreY, float x, float y)
        {
            if (x < Pass.minX || x > Pass.maxX || y < Pass.minY || y > Pass.maxY)
            {
                return false;
            }

            if (Walls.Blocks(push, fromX, fromY, x, y))
            {
                return false;
            }

            if (push.flyer)
            {
                return Terrain.IsFlyable(x, y);
            }

            if (Decks.count > 0)
            {
                float to;
                float from;
                var onTo = OnDeck(x, y, out to);
                var onFrom = OnDeck(fromX, fromY, out from);
                if (onFrom)
                {
                    if (!Inside(push.size, fromX, fromY, centreX, centreY, from))
                    {
                        // Its body would go over the rail.
                        return false;
                    }

                    if (onTo)
                    {
                        return true;
                    }

                    // Off the deck's edge: onto the ground only near the deck's height, else it is the rail.
                    return Near(Terrain.Height(x, y), from) && Terrain.IsWalkable(x, y) && Terrain.ItemLands(x, y);
                }

                if (onTo)
                {
                    // Onto a deck from the ground: only where the deck is near the ground's height.
                    return Near(to, Terrain.Height(fromX, fromY));
                }
            }

            return Terrain.IsWalkable(x, y) && Terrain.ItemLands(x, y);
        }

        /// <summary>
        /// Whether a unit on a deck of a height at (fromX, fromY) keeps its body inside the deck's rail when it moves to
        /// (centreX, centreY): each point of its edge, its collision size out every eighth of a turn, on the side it moves
        /// toward, is on a deck or over ground within DeckStep of the deck, or was off both already where it is (a body
        /// over the rail already does not hold it there, so it can be pushed back or along).
        /// </summary>
        private static bool Inside(float size, float fromX, float fromY, float centreX, float centreY, float deck)
        {
            var moveX = centreX - fromX;
            var moveY = centreY - fromY;
            var slant = size * Slant;
            return Rim(moveX, moveY, size, 0.0f, fromX, fromY, centreX, centreY, deck)
                && Rim(moveX, moveY, -size, 0.0f, fromX, fromY, centreX, centreY, deck)
                && Rim(moveX, moveY, 0.0f, size, fromX, fromY, centreX, centreY, deck)
                && Rim(moveX, moveY, 0.0f, -size, fromX, fromY, centreX, centreY, deck)
                && Rim(moveX, moveY, slant, slant, fromX, fromY, centreX, centreY, deck)
                && Rim(moveX, moveY, slant, -slant, fromX, fromY, centreX, centreY, deck)
                && Rim(moveX, moveY, -slant, slant, fromX, fromY, centreX, centreY, deck)
                && Rim(moveX, moveY, -slant, -slant, fromX, fromY, centreX, centreY, deck);
        }

        /// <summary>Whether one point of a unit's edge, (offsetX, offsetY) from its middle, does not take it over a deck's rail: on a side it does not move toward, held where it moves to, or not held where it is already.</summary>
        private static bool Rim(float moveX, float moveY, float offsetX, float offsetY, float fromX, float fromY, float centreX, float centreY, float deck)
        {
            if (moveX * offsetX + moveY * offsetY <= 0.0f)
            {
                return true;
            }

            return Holds(centreX + offsetX, centreY + offsetY, deck) || !Holds(fromX + offsetX, fromY + offsetY, deck);
        }

        /// <summary>Whether a point holds the body of a unit on a deck of a height: it is on a deck, or the ground there is within DeckStep of that height.</summary>
        private static bool Holds(float x, float y, float deck)
        {
            float height;
            return OnDeck(x, y, out height) || Near(Terrain.Height(x, y), deck);
        }

        /// <summary>Whether a point is on a deck, which is above the terrain there, and the deck's height; false off every deck, or where the terrain is higher (a deck's end, in a bank).</summary>
        private static bool OnDeck(float x, float y, out float height)
        {
            return Decks.Under(x, y, out height) && height >= Terrain.Height(x, y);
        }

        /// <summary>Whether two heights are within DeckStep.</summary>
        private static bool Near(float a, float b)
        {
            var apart = a - b;
            return apart <= DeckStep && apart >= -DeckStep;
        }

        /// <summary>Puts a unit at a place with SetUnitX/Y, keeping its order, and in its cell of the grid at once.</summary>
        private static void Place(Unit unit, float x, float y)
        {
            unit.PlaceAt(x, y);
            Tracking.UnitMoved(unit, x, y);
        }

        /// <summary>Runs OnSlammed for a slam: the speed lost, the unit's place, and the direction out of what it hit.</summary>
        private static void Slammed(Pushed push, float speed, float x, float y, float nx, float ny)
        {
            if (speed <= 0.0f)
            {
                return;
            }

            React(push, SlammedBit, null, new Slam(speed, x, y, nx, ny, push.pusher), 0.0f);
        }

        /// <summary>A lifted unit that does not fly, for a step: gravity pulls it down, and it comes down (OnGrounded, with the speed it fell at) when its height reaches its own, its type's fly height; pushed down while at its height, it stays there.</summary>
        private static void Fall(Pushed push, float step)
        {
            var gravity = Physics.Current().Gravity;
            var height = push.height;
            var home = push.home;
            var vz = push.vz;
            var next = vz - gravity * step;
            var to = height + (vz + next) * 0.5f * step;
            if (to > home)
            {
                push.height = to;
                push.vz = next;
                push.unit.FlyHeight = to;
                return;
            }

            push.height = home;
            push.vz = 0.0f;
            push.lifted = false;
            push.unit.FlyHeight = home;
            if (height > home)
            {
                var square = vz * vz + 2.0f * gravity * (height - home);
                React(push, GroundedBit, null, null, square > 0.0f ? Trig.Sqrt(square) : 0.0f);
            }
        }

        /// <summary>A lifted flyer, for a step: drawn back toward its type's height at its Body.Friction a second each second, and settled there when it comes back to it; it never lands.</summary>
        private static void Drift(Pushed push, float step)
        {
            var pull = Friction(push);
            var height = push.height;
            var vz = push.vz;
            var off = height - push.home;
            var toward = off > 0.0f || (off == 0.0f && vz > 0.0f) ? -pull : pull;
            var next = vz + toward * step;
            var to = height + (vz + next) * 0.5f * step;
            var offNext = to - push.home;
            if ((toward < 0.0f && offNext <= 0.0f && next <= 0.0f) || (toward > 0.0f && offNext >= 0.0f && next >= 0.0f))
            {
                to = push.home;
                next = 0.0f;
                push.lifted = false;
            }

            if (to < 0.0f)
            {
                to = 0.0f;
                if (next < 0.0f)
                {
                    next = 0.0f;
                }
            }

            push.height = to;
            push.vz = next;
            push.unit.FlyHeight = to;
        }

        /// <summary>Runs a reaction on the unit's class, then its components and buffs in the order they were added, with the unit as the current owner; only where the class writes it, and not on what has ended. A failing one is reported and the rest run; once one has killed, hidden, loaded or removed the unit, none after it runs.</summary>
        private static void React(Pushed push, int bit, Unit? pusher, Slam? slam, float speed)
        {
            var unit = push.unit;
            reacted = true;
            var previous = Owner.current;
            Owner.current = unit;
            if (!unit.HasEnded && (MaskOf(unit) & bit) != 0)
            {
                Call(unit, bit, pusher, slam, speed);
            }

            var count = unit.BeginDispatch();
            for (var i = 0; i < count; i++)
            {
                var component = unit.ComponentAt(i) as UnitComponent;
                if (component != null && !component.HasEnded && (MaskOf(component) & bit) != 0)
                {
                    if (Gone(unit))
                    {
                        break;
                    }

                    Call(component, bit, pusher, slam, speed);
                }
            }

            unit.EndDispatch();
            Owner.current = previous;
        }

        /// <summary>Calls a reaction on an object that writes it, reporting an error it throws.</summary>
        private static void Call(object obj, int bit, Unit? pusher, Slam? slam, float speed)
        {
            try
            {
                var target = (IPushTarget)obj;
                if (bit == PushedBit)
                {
                    target.OnPushed(pusher);
                }
                else if (bit == SlammedBit)
                {
                    target.OnSlammed(slam!);
                }
                else
                {
                    target.OnGrounded(speed);
                }
            }
            catch (Exception error)
            {
                var name = bit == PushedBit ? ".OnPushed" : (bit == SlammedBit ? ".OnSlammed" : ".OnGrounded");
                Errors.Report(Names.NameOf(obj) + name, error);
            }
        }

        /// <summary>The bits of the reactions an object's class writes, worked out once per class. Only methods the class writes count: in Lua an interface's empty defaults are not there.</summary>
        private static int MaskOf(object obj)
        {
            if (masks == null)
            {
                masks = new Dictionary<object, int>();
            }

            var key = Names.ClassOf(obj);
            if (masks.TryGetValue(key, out var mask))
            {
                return mask;
            }

            mask = 0;
            if (obj is IPushTarget)
            {
                if (Hooks.Overrides(obj, "OnPushed", typeof(object)))
                {
                    mask |= PushedBit;
                }

                if (Hooks.Overrides(obj, "OnSlammed", typeof(object)))
                {
                    mask |= SlammedBit;
                }

                if (Hooks.Overrides(obj, "OnGrounded", typeof(object)))
                {
                    mask |= GroundedBit;
                }
            }

            masks[key] = mask;
            return mask;
        }

        /// <summary>Takes the ended pushes out of the list, keeping the order of the rest.</summary>
        private static void Pack(List<Pushed> list)
        {
            dropped = false;
            var kept = 0;
            var count = list.Count;
            for (var i = 0; i < count; i++)
            {
                var push = Raw.At(list, i);
                if (!push.ended)
                {
                    Raw.Put(list, kept, push);
                    kept++;
                }
            }

            Raw.Truncate(list, kept);
        }
    }
}
