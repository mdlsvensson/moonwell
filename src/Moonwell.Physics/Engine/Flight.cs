using System;
using System.Collections.Generic;
using Moonwell.Hosting;

namespace Moonwell.Engine
{
    /// <summary>A missile's step, its launch from a unit, and its effects. The step works in floats and makes no table.</summary>
    internal static class Flight
    {
        /// <summary>A missile below this height ends.</summary>
        private const float Lowest = -2000.0f;

        /// <summary>A missile above this height ends.</summary>
        private const float Highest = 3000.0f;

        /// <summary>A mark nearer than this along the ground is at the launch's own place: the missile flies along the source's facing.</summary>
        private const float SamePlace = 1.0f;

        /// <summary>The share of a step a Duration or MaxDistance is forgiven, as a sum of steps can come out a little under in a float.</summary>
        private const float Forgiven = 0.001f;

        /// <summary>The ground's height at a point: the one place the package reads it, so that the decks of bridges can join it.</summary>
        public static float Ground(float x, float y)
        {
            return Terrain.Height(x, y);
        }

        /// <summary>
        /// Moves a missile one step: the speed change, gravity, then the move, added exactly for the step; a missile that
        /// follows the ground keeps its height above it. Then its Duration and MaxDistance, which end it after OnExpire, and
        /// the world's bounds, which end it. Then its effects are placed and turned.
        /// </summary>
        public static void Step(Missile missile, float step, Grid grid, IPhysicsWorld world)
        {
            var vx = missile.vx;
            var vy = missile.vy;
            var vz = missile.vz;
            var acceleration = missile.acceleration;
            if (acceleration != 0.0f)
            {
                var speed = Trig.Sqrt(vx * vx + vy * vy + vz * vz);
                if (speed > 0.0f)
                {
                    var next = Faster(speed, acceleration * step, missile.maxSpeed);
                    var share = next / speed;
                    vx *= share;
                    vy *= share;
                    vz *= share;
                }
            }

            var gravity = missile.gravity;
            var rise = vz * step - gravity * step * step * 0.5f;
            vz -= gravity * step;

            // The components' turns (Homing) come here.
            missile.vx = vx;
            missile.vy = vy;
            missile.vz = vz;

            var entry = missile.entry!;
            var fromX = entry.X;
            var fromY = entry.Y;
            var fromZ = missile.z;
            var x = fromX + vx * step;
            var y = fromY + vy * step;
            var z = fromZ + rise;
            if (missile.followsGround)
            {
                var below = missile.groundRead && missile.groundX == fromX && missile.groundY == fromY ? missile.groundZ : Ground(fromX, fromY);
                var ground = Ground(x, y);
                z = ground + z - below;
                missile.groundRead = true;
                missile.groundX = x;
                missile.groundY = y;
                missile.groundZ = ground;
            }

            grid.Move(entry, x, y);
            missile.z = z;

            var dx = x - fromX;
            var dy = y - fromY;
            var dz = z - fromZ;
            missile.flown += step;
            missile.travelled += Trig.Sqrt(dx * dx + dy * dy + dz * dz);
            var duration = missile.duration;
            var maxDistance = missile.maxDistance;
            if ((duration > 0.0f && missile.flown >= duration - step * Forgiven) || (maxDistance > 0.0f && missile.travelled >= maxDistance - Trig.Sqrt(vx * vx + vy * vy + vz * vz) * step * Forgiven))
            {
                Expire(missile);
                return;
            }

            if (x < Pass.minX || x > Pass.maxX || y < Pass.minY || y > Pass.maxY || z < Lowest || z > Highest)
            {
                missile.End();
                return;
            }

            // The ground comes here.
            Place(missile, world, x, y, z);
        }

        /// <summary>A speed after a change: down to 0 at the least, and up to the cap when there is one, unless it was over it already.</summary>
        private static float Faster(float speed, float change, float cap)
        {
            var next = speed + change;
            if (next < 0.0f)
            {
                return 0.0f;
            }

            if (change > 0.0f && cap > 0.0f && next > cap)
            {
                return speed > cap ? speed : cap;
            }

            return next;
        }

        /// <summary>Runs OnExpire with the missile as the current owner, reporting an error it throws, and ends the missile.</summary>
        private static void Expire(Missile missile)
        {
            var previous = Owner.current;
            Owner.current = missile;
            try
            {
                missile.OnExpire();
            }
            catch (Exception error)
            {
                Errors.Report(Names.NameOf(missile) + ".OnExpire", error);
            }

            Owner.current = previous;
            missile.End();
        }

        /// <summary>
        /// Launches a missile from a unit toward a mark and creates it. It starts at the unit's place, its LaunchOffset ahead
        /// along the unit's facing, and its LaunchHeight above the ground and the unit's fly height there. The mark is
        /// <paramref name="above"/> over the ground at (<paramref name="x"/>, <paramref name="y"/>); when
        /// <paramref name="level"/>, it is at the launch's own height above the ground, or on the ground with an Arc. The
        /// source and its player are the missile's.
        /// </summary>
        public static void Launch(Unit source, Missile missile, float x, float y, float above, bool level, Owner? owner)
        {
            Physics.EnsureInstalled();
            var facing = source.Facing;
            var sourceX = source.X;
            var sourceY = source.Y;
            var offset = missile.launchOffset;
            var startX = sourceX + offset * Angles.Cos(facing);
            var startY = sourceY + offset * Angles.Sin(facing);
            var startAbove = source.FlyHeight + missile.launchHeight;
            var startZ = Ground(sourceX, sourceY) + startAbove;
            if (level)
            {
                above = missile.arc > 0.0f ? 0.0f : startAbove;
            }

            var dz = missile.followsGround ? above - startAbove : Ground(x, y) + above - startZ;
            Aim(missile, x - startX, y - startY, dz, facing);
            missile.source = source;
            Missile.Create(missile, startX, startY, startZ, source.Player, owner);
        }

        /// <summary>
        /// Sets a missile's velocity at its Speed toward a mark a way off. With an Arc it flies along the ground at its Speed
        /// and is given the Gravity and lift that peak at Arc times the distance above the line to the mark and land on it,
        /// as the game's Missile Arc; without one it flies straight at the mark. A mark at its own place sends it level along
        /// the facing, in degrees.
        /// </summary>
        private static void Aim(Missile missile, float dx, float dy, float dz, float facing)
        {
            var speed = missile.Speed;
            var distance = Trig.Sqrt(dx * dx + dy * dy);
            if (distance < SamePlace)
            {
                missile.vx = speed * Angles.Cos(facing);
                missile.vy = speed * Angles.Sin(facing);
                missile.vz = 0.0f;
                return;
            }

            var arc = missile.arc;
            if (arc > 0.0f && speed > 0.0f)
            {
                var time = distance / speed;
                var peak = arc * distance;
                missile.gravity = 8.0f * peak / (time * time);
                missile.vx = dx / time;
                missile.vy = dy / time;
                missile.vz = dz / time + 4.0f * peak / time;
                return;
            }

            var share = speed / Trig.Sqrt(distance * distance + dz * dz);
            missile.vx = dx * share;
            missile.vy = dy * share;
            missile.vz = dz * share;
        }

        /// <summary>Makes a missile's effects at a place, at its height, as it is created: its model's, then those AddEffect listed.</summary>
        public static void Show(Missile missile, float x, float y)
        {
            missile.shown = true;
            var world = Physics.World;
            MakeLook(missile, world, x, y);
            var models = missile.extraModels;
            if (models != null)
            {
                for (var i = 0; i < models.Count; i++)
                {
                    ShowExtra(missile, i, x, y, world);
                }
            }
        }

        /// <summary>Makes the effect AddEffect listed at an index, where the missile is, after Create.</summary>
        public static void ShowExtra(Missile missile, int index)
        {
            ShowExtra(missile, index, missile.X, missile.Y, Physics.World);
        }

        /// <summary>Destroys a missile's model's effect and makes it again of its Model, where it is, with its scale, tint and facing.</summary>
        public static void Remake(Missile missile)
        {
            var world = Physics.World;
            var old = missile.look;
            missile.look = null;
            if (old != null)
            {
                world.RemoveEffect(old);
            }

            MakeLook(missile, world, missile.X, missile.Y);
        }

        /// <summary>Destroys a missile's effects; nothing is shown after.</summary>
        public static void Hide(Missile missile)
        {
            if (!missile.shown)
            {
                return;
            }

            missile.shown = false;
            var world = Physics.World;
            var look = missile.look;
            missile.look = null;
            if (look != null)
            {
                world.RemoveEffect(look);
            }

            var extras = missile.extras;
            if (extras != null)
            {
                for (var i = 0; i < extras.Count; i++)
                {
                    world.RemoveEffect(extras[i]);
                }

                extras.Clear();
            }
        }

        /// <summary>Gives an effect a missile's tint.</summary>
        public static void Colour(Missile missile, effect target)
        {
            var world = Physics.World;
            world.SetEffectColor(target, missile.tintRed, missile.tintGreen, missile.tintBlue);
            world.SetEffectAlpha(target, missile.tintAlpha);
        }

        /// <summary>Puts a missile's effects at its place, each at its height above it, and turns them to the velocity when it faces its flight.</summary>
        private static void Place(Missile missile, IPhysicsWorld world, float x, float y, float z)
        {
            var look = missile.look;
            if (look != null)
            {
                world.SetEffectPosition(look, x, y, z + missile.modelHeight);
            }

            var extras = missile.extras;
            if (extras != null)
            {
                var heights = missile.extraHeights!;
                var count = extras.Count;
                for (var i = 0; i < count; i++)
                {
                    world.SetEffectPosition(Raw.At(extras, i), x, y, z + Raw.At(heights, i));
                }
            }

            if (missile.facesFlight)
            {
                Face(missile, world);
            }
        }

        /// <summary>Turns a missile's effects to its velocity: yaw 0 east, pitch positive nose down, in radians. A still missile, or one whose velocity has not changed since, is left as it is.</summary>
        private static void Face(Missile missile, IPhysicsWorld world)
        {
            var vx = missile.vx;
            var vy = missile.vy;
            var vz = missile.vz;
            if ((vx == 0.0f && vy == 0.0f && vz == 0.0f) || (missile.faced && vx == missile.facedX && vy == missile.facedY && vz == missile.facedZ))
            {
                return;
            }

            var yaw = Trig.Atan2(vy, vx);
            var pitch = -Trig.Atan2(vz, Trig.Sqrt(vx * vx + vy * vy));
            missile.faced = true;
            missile.facedX = vx;
            missile.facedY = vy;
            missile.facedZ = vz;
            missile.facedYaw = yaw;
            missile.facedPitch = pitch;
            var look = missile.look;
            if (look != null)
            {
                world.SetEffectOrientation(look, yaw, pitch, 0.0f);
            }

            var extras = missile.extras;
            if (extras != null)
            {
                var count = extras.Count;
                for (var i = 0; i < count; i++)
                {
                    world.SetEffectOrientation(Raw.At(extras, i), yaw, pitch, 0.0f);
                }
            }
        }

        /// <summary>Makes a missile's model's effect at a place, at its model height, with its scale, tint and facing; none with no model.</summary>
        private static void MakeLook(Missile missile, IPhysicsWorld world, float x, float y)
        {
            if (missile.model.Length == 0)
            {
                return;
            }

            var look = world.AddEffect(missile.model, x, y);
            missile.look = look;
            world.SetEffectPosition(look, x, y, missile.z + missile.modelHeight);
            if (missile.scale != 1.0f)
            {
                world.SetEffectScale(look, missile.scale);
            }

            if (missile.tinted)
            {
                Colour(missile, look);
            }

            Turn(missile, world, look);
        }

        /// <summary>Makes the effect AddEffect listed at an index, at a place, at its height, with the missile's facing.</summary>
        private static void ShowExtra(Missile missile, int index, float x, float y, IPhysicsWorld world)
        {
            var made = world.AddEffect(missile.extraModels![index], x, y);
            world.SetEffectPosition(made, x, y, missile.z + missile.extraHeights![index]);
            missile.extras!.Add(made);
            Turn(missile, world, made);
        }

        /// <summary>Turns a new effect to the missile's facing: the last it was turned to, or its velocity when it has not been turned and has one.</summary>
        private static void Turn(Missile missile, IPhysicsWorld world, effect target)
        {
            if (!missile.facesFlight)
            {
                return;
            }

            if (!missile.faced)
            {
                Face(missile, world);
                return;
            }

            world.SetEffectOrientation(target, missile.facedYaw, missile.facedPitch, 0.0f);
        }
    }
}
