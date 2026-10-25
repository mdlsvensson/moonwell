using Moonwell.Hosting;

namespace Moonwell.Engine
{
    /// <summary>
    /// The game as the terrain package sees it, through the game's natives; the only class of the package that calls
    /// them. The location behind <c>LocationZ</c> is made when the world is made, which is in <c>Terrain.Install</c>, in
    /// synced code: <c>LocationZ</c> is machine-local, a map calls it only inside <c>Local.For</c>, and making a handle
    /// there would make it on one machine only.
    /// </summary>
    internal sealed class NativeTerrainWorld : ITerrainWorld
    {
        /// <summary>The location LocationZ moves about: one handle, made at install, for the map's life.</summary>
        private readonly location spot;

        /// <summary>The item Place puts down, made on first use.</summary>
        private item? marker;

        /// <summary>Makes the world, and the location LocationZ moves about.</summary>
        internal NativeTerrainWorld()
        {
            spot = Natives.Location(0f, 0f);
        }

        /// <summary>The ground's height at a point, as the game gives it on this machine.</summary>
        public float LocationZ(float x, float y)
        {
            Natives.MoveLocation(spot, x, y);
            return Natives.GetLocationZ(spot);
        }

        /// <summary>Whether units can walk at a point: the game says whether the pathing type is off there, so the answer is turned round.</summary>
        public bool IsWalkable(float x, float y) => !Natives.IsTerrainPathable(x, y, Natives.PATHING_TYPE_WALKABILITY);

        /// <summary>Whether flying units can fly at a point.</summary>
        public bool IsFlyable(float x, float y) => !Natives.IsTerrainPathable(x, y, Natives.PATHING_TYPE_FLYABILITY);

        /// <summary>Whether a building can be built at a point.</summary>
        public bool IsBuildable(float x, float y) => !Natives.IsTerrainPathable(x, y, Natives.PATHING_TYPE_BUILDABILITY);

        /// <summary>Whether ships can float at a point.</summary>
        public bool IsFloatable(float x, float y) => !Natives.IsTerrainPathable(x, y, Natives.PATHING_TYPE_FLOATABILITY);

        /// <summary>Puts one item down at the point, shown for a moment, and reads where the game left it.</summary>
        public void Place(float x, float y, out float landedX, out float landedY)
        {
            if (marker == null)
            {
                marker = Natives.CreateItem(ObjectTypes.FourCC("wolg"), x, y);
            }

            Natives.SetItemVisible(marker, true);
            Natives.SetItemPosition(marker, x, y);
            landedX = Natives.GetItemX(marker);
            landedY = Natives.GetItemY(marker);
            Natives.SetItemVisible(marker, false);
        }

        /// <summary>The playable area, as the game's own bj_mapInitialPlayableArea is made: the camera's bounds, less the camera's margins.</summary>
        public void PlayableArea(out float minX, out float minY, out float maxX, out float maxY)
        {
            minX = Natives.GetCameraBoundMinX() - Natives.GetCameraMargin(Natives.CAMERA_MARGIN_LEFT);
            minY = Natives.GetCameraBoundMinY() - Natives.GetCameraMargin(Natives.CAMERA_MARGIN_BOTTOM);
            maxX = Natives.GetCameraBoundMaxX() + Natives.GetCameraMargin(Natives.CAMERA_MARGIN_RIGHT);
            maxY = Natives.GetCameraBoundMaxY() + Natives.GetCameraMargin(Natives.CAMERA_MARGIN_TOP);
        }
    }
}
