using System.Collections.Generic;
using Moonwell.Hosting;

namespace Moonwell.Testing
{
    /// <summary>The fake game the physics package's cases run against: effects, images and sounds that are only recorded, a world's bounds, and the facts about units and placed destructables a case sets.</summary>
    public sealed class FakePhysicsWorld : IPhysicsWorld
    {
        /// <summary>The world's left edge.</summary>
        public float MinX = -4096.0f;

        /// <summary>The world's bottom edge.</summary>
        public float MinY = -4096.0f;

        /// <summary>The world's right edge.</summary>
        public float MaxX = 4096.0f;

        /// <summary>The world's top edge.</summary>
        public float MaxY = 4096.0f;

        /// <summary>The effects made and not yet removed, in the order they were made.</summary>
        public List<effect> Effects = new List<effect>();

        /// <summary>The images made and not yet removed, in the order they were made.</summary>
        public List<image> Images = new List<image>();

        /// <summary>The looping sounds made and not yet stopped, in the order they were made.</summary>
        public List<sound> Loops = new List<sound>();

        /// <summary>The paths of the sounds played once, in the order they were played.</summary>
        public List<string> Played = new List<string>();

        /// <summary>The yaw, pitch and roll, in radians, last given to each effect, in the order they were given: three numbers to a call.</summary>
        public List<float> Orientations = new List<float>();

        /// <summary>The collision size of each unit a case gave one; a unit with none reads 16.</summary>
        public Dictionary<unit, float> Sizes = new Dictionary<unit, float>();

        /// <summary>The units IsStructure answers true for.</summary>
        public List<unit> Structures = new List<unit>();

        /// <summary>The units IsFlying answers true for.</summary>
        public List<unit> Flyers = new List<unit>();

        /// <summary>The x of each unit PlaceUnitX moved, in the order of the calls.</summary>
        public List<float> PlacedX = new List<float>();

        /// <summary>The y of each unit PlaceUnitY moved, in the order of the calls.</summary>
        public List<float> PlacedY = new List<float>();

        /// <summary>What FindPlacedDestructable gives for a type; a type with none gives null.</summary>
        public Dictionary<int, destructable> Placed = new Dictionary<int, destructable>();

        /// <summary>How many times the package read the world's bounds.</summary>
        public int BoundsReads;

        /// <summary>Makes an effect.</summary>
        public effect AddEffect(string model, float x, float y)
        {
            var made = new effect();
            Effects.Add(made);
            return made;
        }

        /// <summary>Puts an effect at a place; nothing is kept.</summary>
        public void SetEffectPosition(effect target, float x, float y, float z)
        {
        }

        /// <summary>Records the angles given.</summary>
        public void SetEffectOrientation(effect target, float yaw, float pitch, float roll)
        {
            Orientations.Add(yaw);
            Orientations.Add(pitch);
            Orientations.Add(roll);
        }

        /// <summary>Scales an effect; nothing is kept.</summary>
        public void SetEffectScale(effect target, float scale)
        {
        }

        /// <summary>Tints an effect; nothing is kept.</summary>
        public void SetEffectColor(effect target, int red, int green, int blue)
        {
        }

        /// <summary>Sets an effect's opacity; nothing is kept.</summary>
        public void SetEffectAlpha(effect target, int alpha)
        {
        }

        /// <summary>Removes an effect.</summary>
        public void RemoveEffect(effect target)
        {
            Effects.Remove(target);
        }

        /// <summary>Makes an image.</summary>
        public image CreateShadow(string file, float size, float x, float y)
        {
            var made = new image();
            Images.Add(made);
            return made;
        }

        /// <summary>Moves an image; nothing is kept.</summary>
        public void MoveImage(image target, float size, float x, float y)
        {
        }

        /// <summary>Sets an image's opacity; nothing is kept.</summary>
        public void SetImageAlpha(image target, int alpha)
        {
        }

        /// <summary>Removes an image.</summary>
        public void RemoveImage(image target)
        {
            Images.Remove(target);
        }

        /// <summary>Makes a looping sound.</summary>
        public sound CreateLoopingSound(string path, float x, float y, float z)
        {
            var made = new sound();
            Loops.Add(made);
            return made;
        }

        /// <summary>Moves a sound; nothing is kept.</summary>
        public void MoveSound(sound target, float x, float y, float z)
        {
        }

        /// <summary>Sets a looping sound's volume; nothing is kept.</summary>
        public void SetLoopVolume(sound target, int volume)
        {
        }

        /// <summary>Stops a looping sound.</summary>
        public void StopLoop(sound target)
        {
            Loops.Remove(target);
        }

        /// <summary>Records the path of a sound played once.</summary>
        public void PlaySoundAt(string path, float x, float y, float z)
        {
            Played.Add(path);
        }

        /// <summary>A unit's collision size: the one a case gave it, else 16.</summary>
        public float CollisionSize(unit target)
        {
            float size;
            if (Sizes.TryGetValue(target, out size))
            {
                return size;
            }

            return 16.0f;
        }

        /// <summary>Whether a unit is a structure.</summary>
        public bool IsStructure(unit target) => Structures.Contains(target);

        /// <summary>Whether a unit flies.</summary>
        public bool IsFlying(unit target) => Flyers.Contains(target);

        /// <summary>Records the x a unit was put at.</summary>
        public void PlaceUnitX(unit target, float x)
        {
            PlacedX.Add(x);
        }

        /// <summary>Records the y a unit was put at.</summary>
        public void PlaceUnitY(unit target, float y)
        {
            PlacedY.Add(y);
        }

        /// <summary>The destructable a case gave for a type, else null.</summary>
        public destructable? FindPlacedDestructable(int type, float x, float y)
        {
            destructable? found;
            if (Placed.TryGetValue(type, out found))
            {
                return found;
            }

            return null;
        }

        /// <summary>The world's bounds; counts the read.</summary>
        public void WorldBounds(out float minX, out float minY, out float maxX, out float maxY)
        {
            BoundsReads++;
            minX = MinX;
            minY = MinY;
            maxX = MaxX;
            maxY = MaxY;
        }
    }
}
