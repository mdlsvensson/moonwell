using System.Collections.Generic;
using Moonwell.Engine;
using Moonwell.Hosting;

namespace Moonwell
{
    /// <summary>
    /// A shadow on the ground under a missile: <c>Components.Add(new Shadow())</c>. An image of the game's unit shadow, twice
    /// the missile's Radius across, moved with it and fading as it rises: at its Alpha with its lowest point on the ground,
    /// and gone at about 21 times its Size above it (ALICE's fade). The ground under it is read every 0.1 second, or taken
    /// from the pass's own read when it has one there. The image is destroyed when the shadow is removed or the missile ends.
    /// </summary>
    public sealed class Shadow : MissileComponent
    {
        /// <summary>The game's unit shadow, the image unless Path is set.</summary>
        private const string UnitShadow = @"ReplaceableTextures\Shadows\Shadow.blp";

        /// <summary>The alpha lost per game unit above the ground, per unit of Alpha over Size (ALICE's 0.047).</summary>
        private const float Fade = 0.047f;

        /// <summary>The seconds between two reads of the ground under it.</summary>
        private const float Interval = 0.1f;

        /// <summary>The image's file.</summary>
        private string path = UnitShadow;

        /// <summary>The size set; read only when sizeSet.</summary>
        private float size;

        /// <summary>Whether Size has been set.</summary>
        private bool sizeSet;

        /// <summary>The opacity on the ground, 0 to 255.</summary>
        private int alpha = 255;

        /// <summary>The image; null before it is added and after it is removed.</summary>
        internal image? picture;

        /// <summary>The size the image was made with.</summary>
        internal float madeSize;

        /// <summary>The opacity last given to the image; -1 for none.</summary>
        internal int shownAlpha = -1;

        /// <summary>The ground's height under it last read.</summary>
        internal float groundZ;

        /// <summary>The seconds since the ground under it was last read; it starts full, so the first move reads it.</summary>
        internal float since = Interval;

        /// <summary>The image's file: the game's unit shadow unless set. Set after it is added, the image is made again.</summary>
        [LocalSafe]
        public string Path
        {
            get { return path; }
            set
            {
                path = value ?? "";
                Remake();
            }
        }

        /// <summary>How wide the image is: twice the missile's Radius unless set (0 on no missile). Set after it is added, the image is made again. A shadow of size 0 does not fade.</summary>
        [LocalSafe]
        public float Size
        {
            get
            {
                if (sizeSet)
                {
                    return size;
                }

                var missile = actor as Missile;
                return missile == null ? 0.0f : 2.0f * missile.radius;
            }
            set
            {
                Dev.Check(value > 0.0f, "Shadow.Size: must be more than 0");
                size = value;
                sizeSet = true;
                Remake();
            }
        }

        /// <summary>The opacity with the missile on the ground, 0 to 255; 255 unless set. It fades from this as the missile rises.</summary>
        [LocalSafe]
        public int Alpha
        {
            get { return alpha; }
            set
            {
                Dev.Check(value >= 0 && value <= 255, "Shadow.Alpha: 0 to 255");
                alpha = value;
                if (picture != null)
                {
                    var missile = Missile;
                    Show(missile, Physics.World, missile.X, missile.Y, missile.z);
                }
            }
        }

        /// <summary>Makes the image under the missile and joins the missile's shadows.</summary>
        protected override void OnAdded()
        {
            var missile = Missile;
            if (missile.shadows == null)
            {
                missile.shadows = new List<Shadow>();
            }

            missile.shadows.Add(this);
            Make(missile);
        }

        /// <summary>Destroys the image and leaves the missile's shadows; the missile may have left the grid already.</summary>
        protected override void OnRemoved()
        {
            var missile = Missile;
            if (missile.shadows != null)
            {
                missile.shadows.Remove(this);
            }

            Unmake();
        }

        /// <summary>Moves a missile's shadows under a place, and fades each for its height; the pass calls it with the missile's effects, every step it moves.</summary>
        internal static void Follow(Missile missile, IPhysicsWorld world, float x, float y, float z, float step)
        {
            var list = missile.shadows!;
            var count = list.Count;
            for (var i = 0; i < count; i++)
            {
                var shadow = Raw.At(list, i);
                shadow.since += step;
                shadow.Show(missile, world, x, y, z);
            }
        }

        /// <summary>Centres the image on a place and gives it the opacity for the height there; the ground is read when its interval is up and the pass has not read it there.</summary>
        private void Show(Missile missile, IPhysicsWorld world, float x, float y, float z)
        {
            var made = picture;
            if (made == null)
            {
                return;
            }

            var across = madeSize;
            world.MoveImage(made, across, x, y);
            if (missile.groundRead && missile.groundX == x && missile.groundY == y)
            {
                groundZ = Ground.Last(missile);
                since = 0.0f;
            }
            else if (since >= Interval)
            {
                groundZ = Ground.Height(x, y, z);
                since = 0.0f;
            }

            var high = z - missile.radius - groundZ;
            var opacity = alpha;
            if (high > 0.0f && across > 0.0f)
            {
                opacity = alpha - (int)(Fade * alpha / across * high);
                if (opacity < 0)
                {
                    opacity = 0;
                }
            }

            if (opacity != shownAlpha)
            {
                shownAlpha = opacity;
                world.SetImageAlpha(made, opacity);
            }
        }

        /// <summary>Makes the image under the missile, of its file and size, with its opacity.</summary>
        private void Make(Missile missile)
        {
            var world = Physics.World;
            var across = Size;
            var x = missile.X;
            var y = missile.Y;
            madeSize = across;
            picture = world.CreateShadow(path, across, x, y);
            shownAlpha = -1;
            since = Interval;
            Show(missile, world, x, y, missile.z);
        }

        /// <summary>Destroys the image, if there is one.</summary>
        private void Unmake()
        {
            var made = picture;
            picture = null;
            if (made != null)
            {
                Physics.World.RemoveImage(made);
            }
        }

        /// <summary>Makes the image again, of a new file or size, once it has been made.</summary>
        private void Remake()
        {
            if (picture == null)
            {
                return;
            }

            Unmake();
            Make(Missile);
        }
    }
}
