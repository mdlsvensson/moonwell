namespace Moonwell.Hosting
{
    /// <summary>
    /// The game as the physics package sees it: the effects, images and sounds it draws and plays, the facts about a unit it
    /// needs, the destructables placed in the map, and the world's bounds. Types are the game's. Angles of an effect are in
    /// radians, as the natives take them; the rest of Moonwell speaks degrees, so the package converts.
    /// </summary>
    public interface IPhysicsWorld
    {
        /// <summary>Makes an effect of a model at a place on the ground.</summary>
        effect AddEffect(string model, float x, float y);

        /// <summary>Puts an effect at a place and height.</summary>
        void SetEffectPosition(effect target, float x, float y, float z);

        /// <summary>Turns an effect: yaw, pitch and roll in radians.</summary>
        void SetEffectOrientation(effect target, float yaw, float pitch, float roll);

        /// <summary>Sets an effect's scale; 1 is its own size.</summary>
        void SetEffectScale(effect target, float scale);

        /// <summary>Tints an effect; each of red, green and blue is 0 to 255.</summary>
        void SetEffectColor(effect target, int red, int green, int blue);

        /// <summary>Sets an effect's opacity, 0 to 255.</summary>
        void SetEffectAlpha(effect target, int alpha);

        /// <summary>Destroys an effect: a death animation plays where it stands. The game's DestroyEffect; named otherwise because a method named like a native it calls would call itself in Lua.</summary>
        void RemoveEffect(effect target);

        /// <summary>Makes a shadow image of a square size centred on a place, drawn always, over the ground and under what stands on it.</summary>
        image CreateShadow(string file, float size, float x, float y);

        /// <summary>Moves an image made by CreateShadow so that it is centred on a place; the size is the one it was made with.</summary>
        void MoveImage(image target, float size, float x, float y);

        /// <summary>Sets an image's opacity, 0 to 255.</summary>
        void SetImageAlpha(image target, int alpha);

        /// <summary>Destroys an image. The game's DestroyImage; named otherwise for the same reason as RemoveEffect.</summary>
        void RemoveImage(image target);

        /// <summary>Makes a looping 3D sound at a place and starts it.</summary>
        sound CreateLoopingSound(string path, float x, float y, float z);

        /// <summary>Moves a 3D sound.</summary>
        void MoveSound(sound target, float x, float y, float z);

        /// <summary>Sets a looping sound's volume, 0 to 127. The game's SetSoundVolume; named otherwise for the same reason as RemoveEffect.</summary>
        void SetLoopVolume(sound target, int volume);

        /// <summary>Stops a looping sound and lets the game free it. The game's StopSound; named otherwise for the same reason as RemoveEffect.</summary>
        void StopLoop(sound target);

        /// <summary>Plays a 3D sound once at a place; the game frees it when it ends.</summary>
        void PlaySoundAt(string path, float x, float y, float z);

        /// <summary>A unit's collision size, in game units.</summary>
        float CollisionSize(unit target);

        /// <summary>Whether a unit is a structure.</summary>
        bool IsStructure(unit target);

        /// <summary>Whether a unit flies.</summary>
        bool IsFlying(unit target);

        /// <summary>The fly height of a unit's type, which a flying unit pushed up or down drifts back to: the game's GetUnitDefaultFlyHeight.</summary>
        float DefaultFlyHeight(unit target);

        /// <summary>Puts a unit at an x, keeping its orders. The game's SetUnitX; named otherwise for the same reason as RemoveEffect.</summary>
        void PlaceUnitX(unit target, float x);

        /// <summary>Puts a unit at a y, keeping its orders. The game's SetUnitY; named otherwise for the same reason as RemoveEffect.</summary>
        void PlaceUnitY(unit target, float y);

        /// <summary>
        /// The destructable of a type nearest a place, or null when there is none. A destructable the game made from the
        /// map's placed objects (a bridge) has no handle until the game is asked, so the native world looks in a wide rect
        /// around the place.
        /// </summary>
        destructable? FindPlacedDestructable(int type, float x, float y);

        /// <summary>The world's bounds, read once.</summary>
        void WorldBounds(out float minX, out float minY, out float maxX, out float maxY);
    }
}
