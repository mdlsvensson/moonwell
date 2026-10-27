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

        /// <summary>The file each image was made of.</summary>
        public Dictionary<image, string> ImageFiles = new Dictionary<image, string>();

        /// <summary>The size each image was made with.</summary>
        public Dictionary<image, float> ImageSizes = new Dictionary<image, float>();

        /// <summary>The x and y each image was last centred on.</summary>
        public Dictionary<image, float[]> ImagePlaces = new Dictionary<image, float[]>();

        /// <summary>The opacity each image was last given; one never given one has none.</summary>
        public Dictionary<image, int> ImageAlphas = new Dictionary<image, int>();

        /// <summary>The images removed, in the order they were removed.</summary>
        public List<image> RemovedImages = new List<image>();

        /// <summary>The path each looping sound was made of.</summary>
        public Dictionary<sound, string> SoundPaths = new Dictionary<sound, string>();

        /// <summary>The x, y and z each looping sound was last put at.</summary>
        public Dictionary<sound, float[]> SoundPlaces = new Dictionary<sound, float[]>();

        /// <summary>The volume, 0 to 127, each looping sound was last given; one never given one has none.</summary>
        public Dictionary<sound, int> Volumes = new Dictionary<sound, int>();

        /// <summary>How many times a looping sound was moved.</summary>
        public int SoundMoves;

        /// <summary>The looping sounds stopped, in the order they were stopped.</summary>
        public List<sound> Stopped = new List<sound>();

        /// <summary>The paths of the sounds played once, in the order they were played.</summary>
        public List<string> Played = new List<string>();

        /// <summary>The yaw, pitch and roll, in radians, last given to each effect, in the order they were given: three numbers to a call.</summary>
        public List<float> Orientations = new List<float>();

        /// <summary>The model each effect was made of.</summary>
        public Dictionary<effect, string> Models = new Dictionary<effect, string>();

        /// <summary>The x, y and z each effect was last put at; one never put has none.</summary>
        public Dictionary<effect, float[]> Places = new Dictionary<effect, float[]>();

        /// <summary>The x and y each effect was made at, which the game draws its trail from.</summary>
        public Dictionary<effect, float[]> MadeAt = new Dictionary<effect, float[]>();

        /// <summary>The scale each effect was last given; one never scaled has none.</summary>
        public Dictionary<effect, float> Scales = new Dictionary<effect, float>();

        /// <summary>The red, green and blue each effect was last tinted with; one never tinted has none.</summary>
        public Dictionary<effect, int[]> Colors = new Dictionary<effect, int[]>();

        /// <summary>The opacity each effect was last given; one never given one has none.</summary>
        public Dictionary<effect, int> Alphas = new Dictionary<effect, int>();

        /// <summary>The effects removed, in the order they were removed.</summary>
        public List<effect> Removed = new List<effect>();

        /// <summary>The collision size of each unit a case gave one; a unit with none reads 16.</summary>
        public Dictionary<unit, float> Sizes = new Dictionary<unit, float>();

        /// <summary>The units IsStructure answers true for.</summary>
        public List<unit> Structures = new List<unit>();

        /// <summary>The units IsFlying answers true for.</summary>
        public List<unit> Flyers = new List<unit>();

        /// <summary>The fly height of each unit's type a case gave one; a unit with none reads 0.</summary>
        public Dictionary<unit, float> DefaultHeights = new Dictionary<unit, float>();

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
            Models[made] = model;
            Places[made] = new float[] { x, y, 0.0f };
            MadeAt[made] = new float[] { x, y };
            return made;
        }

        /// <summary>Keeps the place an effect was put at.</summary>
        public void SetEffectPosition(effect target, float x, float y, float z)
        {
            Places[target] = new float[] { x, y, z };
        }

        /// <summary>Records the angles given.</summary>
        public void SetEffectOrientation(effect target, float yaw, float pitch, float roll)
        {
            Orientations.Add(yaw);
            Orientations.Add(pitch);
            Orientations.Add(roll);
        }

        /// <summary>Keeps the scale an effect was given.</summary>
        public void SetEffectScale(effect target, float scale)
        {
            Scales[target] = scale;
        }

        /// <summary>Keeps the tint an effect was given.</summary>
        public void SetEffectColor(effect target, int red, int green, int blue)
        {
            Colors[target] = new int[] { red, green, blue };
        }

        /// <summary>Keeps the opacity an effect was given.</summary>
        public void SetEffectAlpha(effect target, int alpha)
        {
            Alphas[target] = alpha;
        }

        /// <summary>Removes an effect, and records it.</summary>
        public void RemoveEffect(effect target)
        {
            Effects.Remove(target);
            Removed.Add(target);
        }

        /// <summary>Makes an image, and keeps its file, size and place.</summary>
        public image CreateShadow(string file, float size, float x, float y)
        {
            var made = new image();
            Images.Add(made);
            ImageFiles[made] = file;
            ImageSizes[made] = size;
            ImagePlaces[made] = new float[] { x, y };
            return made;
        }

        /// <summary>Keeps the place an image was centred on.</summary>
        public void MoveImage(image target, float size, float x, float y)
        {
            ImagePlaces[target] = new float[] { x, y };
        }

        /// <summary>Keeps the opacity an image was given.</summary>
        public void SetImageAlpha(image target, int alpha)
        {
            ImageAlphas[target] = alpha;
        }

        /// <summary>Removes an image, and records it.</summary>
        public void RemoveImage(image target)
        {
            Images.Remove(target);
            RemovedImages.Add(target);
        }

        /// <summary>Makes a looping sound, and keeps its path and place.</summary>
        public sound CreateLoopingSound(string path, float x, float y, float z)
        {
            var made = new sound();
            Loops.Add(made);
            SoundPaths[made] = path;
            SoundPlaces[made] = new float[] { x, y, z };
            return made;
        }

        /// <summary>Keeps the place a sound was put at, and counts the move.</summary>
        public void MoveSound(sound target, float x, float y, float z)
        {
            SoundMoves++;
            SoundPlaces[target] = new float[] { x, y, z };
        }

        /// <summary>Keeps the volume a looping sound was given.</summary>
        public void SetLoopVolume(sound target, int volume)
        {
            Volumes[target] = volume;
        }

        /// <summary>Stops a looping sound, and records it.</summary>
        public void StopLoop(sound target)
        {
            Loops.Remove(target);
            Stopped.Add(target);
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

        /// <summary>The fly height of a unit's type: the one a case gave it, else 0.</summary>
        public float DefaultFlyHeight(unit target)
        {
            float height;
            if (DefaultHeights.TryGetValue(target, out height))
            {
                return height;
            }

            return 0.0f;
        }

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
