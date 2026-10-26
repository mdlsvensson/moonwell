using Moonwell.Hosting;

namespace Moonwell.Engine
{
    /// <summary>The game as the physics package sees it, through the game's natives; the only class of the package that calls them.</summary>
    internal sealed class NativePhysicsWorld : IPhysicsWorld
    {
        /// <summary>How far from a place FindPlacedDestructable looks. A rect of 32 around a placed destructable found nothing in the game, one of 512 found it (game facts, 10-26).</summary>
        private const float FindReach = 512.0f;

        /// <summary>The image type of a ground image drawn over the terrain, as the shadow probe used.</summary>
        private const int GroundImage = 1;

        /// <summary>The type FindPlacedDestructable looks for, while it enumerates.</summary>
        private int findType;

        /// <summary>The place FindPlacedDestructable looks around: x.</summary>
        private float findX;

        /// <summary>The place FindPlacedDestructable looks around: y.</summary>
        private float findY;

        /// <summary>The squared distance of the nearest destructable found so far.</summary>
        private float findBest;

        /// <summary>The nearest destructable found so far; null while none.</summary>
        private destructable? found;

        /// <summary>Makes an effect of a model at a place on the ground.</summary>
        public effect AddEffect(string model, float x, float y) => Natives.AddSpecialEffect(model, x, y);

        /// <summary>Puts an effect at a place and height.</summary>
        public void SetEffectPosition(effect target, float x, float y, float z) => Natives.BlzSetSpecialEffectPosition(target, x, y, z);

        /// <summary>Turns an effect: yaw, pitch and roll in radians.</summary>
        public void SetEffectOrientation(effect target, float yaw, float pitch, float roll) => Natives.BlzSetSpecialEffectOrientation(target, yaw, pitch, roll);

        /// <summary>Sets an effect's scale.</summary>
        public void SetEffectScale(effect target, float scale) => Natives.BlzSetSpecialEffectScale(target, scale);

        /// <summary>Tints an effect.</summary>
        public void SetEffectColor(effect target, int red, int green, int blue) => Natives.BlzSetSpecialEffectColor(target, red, green, blue);

        /// <summary>Sets an effect's opacity.</summary>
        public void SetEffectAlpha(effect target, int alpha) => Natives.BlzSetSpecialEffectAlpha(target, alpha);

        /// <summary>Destroys an effect.</summary>
        public void RemoveEffect(effect target) => Natives.DestroyEffect(target);

        /// <summary>Makes a shadow image, placed by its bottom-left corner, drawn always.</summary>
        public image CreateShadow(string file, float size, float x, float y)
        {
            var made = Natives.CreateImage(file, size, size, 0.0f, x - size / 2.0f, y - size / 2.0f, 0.0f, 0.0f, 0.0f, 0.0f, GroundImage);
            Natives.SetImageRenderAlways(made, true);
            Natives.ShowImage(made, true);
            return made;
        }

        /// <summary>Moves an image so that it is centred on a place.</summary>
        public void MoveImage(image target, float size, float x, float y) => Natives.SetImagePosition(target, x - size / 2.0f, y - size / 2.0f, 0.0f);

        /// <summary>Sets an image's opacity.</summary>
        public void SetImageAlpha(image target, int alpha) => Natives.SetImageColor(target, 255, 255, 255, alpha);

        /// <summary>Destroys an image.</summary>
        public void RemoveImage(image target) => Natives.DestroyImage(target);

        /// <summary>Makes a looping 3D sound at a place and starts it.</summary>
        public sound CreateLoopingSound(string path, float x, float y, float z)
        {
            var made = Natives.CreateSound(path, true, true, false, 10, 10, "DefaultEAXON");
            Natives.SetSoundPosition(made, x, y, z);
            Natives.StartSound(made);
            return made;
        }

        /// <summary>Moves a 3D sound.</summary>
        public void MoveSound(sound target, float x, float y, float z) => Natives.SetSoundPosition(target, x, y, z);

        /// <summary>Sets a looping sound's volume.</summary>
        public void SetLoopVolume(sound target, int volume) => Natives.SetSoundVolume(target, volume);

        /// <summary>Stops a looping sound, fading it out, and lets the game free it.</summary>
        public void StopLoop(sound target) => Natives.StopSound(target, true, true);

        /// <summary>Plays a 3D sound once at a place; the game frees it when it ends.</summary>
        public void PlaySoundAt(string path, float x, float y, float z)
        {
            var made = Natives.CreateSound(path, false, true, false, 10, 10, "DefaultEAXON");
            Natives.SetSoundPosition(made, x, y, z);
            Natives.StartSound(made);
            Natives.KillSoundWhenDone(made);
        }

        /// <summary>A unit's collision size.</summary>
        public float CollisionSize(unit target) => Natives.BlzGetUnitCollisionSize(target);

        /// <summary>Whether a unit is a structure.</summary>
        public bool IsStructure(unit target) => Natives.IsUnitType(target, Natives.UNIT_TYPE_STRUCTURE);

        /// <summary>Whether a unit flies.</summary>
        public bool IsFlying(unit target) => Natives.IsUnitType(target, Natives.UNIT_TYPE_FLYING);

        /// <summary>The fly height of a unit's type.</summary>
        public float DefaultFlyHeight(unit target) => Natives.GetUnitDefaultFlyHeight(target);

        /// <summary>Puts a unit at an x, keeping its orders.</summary>
        public void PlaceUnitX(unit target, float x) => Natives.SetUnitX(target, x);

        /// <summary>Puts a unit at a y, keeping its orders.</summary>
        public void PlaceUnitY(unit target, float y) => Natives.SetUnitY(target, y);

        /// <summary>The destructable of a type nearest a place, from a rect of 512 around it; null when there is none.</summary>
        public destructable? FindPlacedDestructable(int type, float x, float y)
        {
            findType = type;
            findX = x;
            findY = y;
            findBest = 0.0f;
            found = null;
            var area = Natives.Rect(x - FindReach, y - FindReach, x + FindReach, y + FindReach);
            Natives.EnumDestructablesInRect(area, null, ConsiderDestructable);
            Natives.RemoveRect(area);
            var result = found;
            found = null;
            return result;
        }

        /// <summary>The world's bounds; GetWorldBounds makes a rect, which is read and left.</summary>
        public void WorldBounds(out float minX, out float minY, out float maxX, out float maxY)
        {
            var bounds = Natives.GetWorldBounds();
            minX = Natives.GetRectMinX(bounds);
            minY = Natives.GetRectMinY(bounds);
            maxX = Natives.GetRectMaxX(bounds);
            maxY = Natives.GetRectMaxY(bounds);
        }

        /// <summary>Keeps the destructable the enumeration is at when it is of the type and nearer than the best so far.</summary>
        private void ConsiderDestructable()
        {
            var candidate = Natives.GetEnumDestructable();
            if (Natives.GetDestructableTypeId(candidate) != findType)
            {
                return;
            }

            var dx = Natives.GetDestructableX(candidate) - findX;
            var dy = Natives.GetDestructableY(candidate) - findY;
            var distance = dx * dx + dy * dy;
            if (found == null || distance < findBest)
            {
                found = candidate;
                findBest = distance;
            }
        }
    }
}
