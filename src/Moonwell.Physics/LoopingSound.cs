using System.Collections.Generic;
using Moonwell.Engine;
using Moonwell.Hosting;

namespace Moonwell
{
    /// <summary>
    /// A looping 3D sound that follows a missile: <c>Components.Add(new LoopingSound(path))</c>. It starts where the missile
    /// is when added, is moved with it each step, and when the missile ends, or the sound is removed, it fades out over
    /// FadeOut and stops. The fade belongs to the game, so it outlives the missile and its scope. The game plays at most 4
    /// of one sound file at once (a claim in game facts), so a volley of 20 hums as 4.
    /// </summary>
    public sealed class LoopingSound : MissileComponent
    {
        /// <summary>The seconds between two steps of a fade.</summary>
        private const float FadeStep = 0.05f;

        /// <summary>The game's loudest volume.</summary>
        private const float Loudest = 127.0f;

        /// <summary>The sound file.</summary>
        private readonly string path;

        /// <summary>The volume, 0 to 100.</summary>
        private float volume = 100.0f;

        /// <summary>The seconds a fade out takes.</summary>
        private float fadeOut = 0.5f;

        /// <summary>The playing sound; null before it is added and once it has stopped.</summary>
        internal sound? playing;

        /// <summary>The fade's timer; null until the sound fades.</summary>
        private Timer? fader;

        /// <summary>The seconds the fade has run.</summary>
        private float faded;

        /// <summary>The game's volume, 0 to 127, the fade starts from.</summary>
        private int fadeFrom;

        /// <summary>The world the sound was made in, for the fade after the missile has ended.</summary>
        private IPhysicsWorld? world;

        /// <summary>A looping sound of a file, as <c>Abilities\Spells\Undead\Possession\PossessionMissileLaunch1.wav</c>.</summary>
        public LoopingSound(string path)
        {
            this.path = path ?? "";
        }

        /// <summary>The sound file.</summary>
        [LocalSafe]
        public string Path
        {
            get { return path; }
        }

        /// <summary>How loud it plays, 0 to 100; 100 unless set.</summary>
        [LocalSafe]
        public float Volume
        {
            get { return volume; }
            set
            {
                Dev.Check(value >= 0.0f && value <= 100.0f, "LoopingSound.Volume: 0 to 100");
                volume = value;
                if (playing != null && fader == null)
                {
                    world!.SetLoopVolume(playing, Level(value));
                }
            }
        }

        /// <summary>The seconds it fades out over after the missile ends; 0.5 unless set, 0 to stop at once.</summary>
        [LocalSafe]
        public float FadeOut
        {
            get { return fadeOut; }
            set
            {
                Dev.Check(value >= 0.0f, "LoopingSound.FadeOut: seconds must be 0 or more");
                fadeOut = value;
            }
        }

        /// <summary>Starts the sound where the missile is, at its volume, and joins the missile's sounds.</summary>
        protected override void OnAdded()
        {
            var missile = Missile;
            if (missile.sounds == null)
            {
                missile.sounds = new List<LoopingSound>();
            }

            missile.sounds.Add(this);
            var made = Physics.World;
            world = made;
            var started = made.CreateLoopingSound(path, missile.X, missile.Y, missile.z);
            playing = started;
            made.SetLoopVolume(started, Level(volume));
        }

        /// <summary>Leaves the missile's sounds and fades the sound out on a timer of the game's, or stops it at once with no FadeOut; the missile may have left the grid already.</summary>
        protected override void OnRemoved()
        {
            var missile = Missile;
            if (missile.sounds != null)
            {
                missile.sounds.Remove(this);
            }

            if (playing == null)
            {
                return;
            }

            if (fadeOut <= 0.0f)
            {
                Stop();
                return;
            }

            faded = 0.0f;
            fadeFrom = Level(volume);
            fader = Game.Instance.Every(FadeStep, () => Fade());
        }

        /// <summary>Moves a missile's sounds to a place; the pass calls it with the missile's effects, every step it moves.</summary>
        internal static void Follow(Missile missile, IPhysicsWorld world, float x, float y, float z)
        {
            var list = missile.sounds!;
            var count = list.Count;
            for (var i = 0; i < count; i++)
            {
                var playing = Raw.At(list, i).playing;
                if (playing != null)
                {
                    world.MoveSound(playing, x, y, z);
                }
            }
        }

        /// <summary>The game's volume, 0 to 127, of a volume 0 to 100, rounded.</summary>
        private static int Level(float volume)
        {
            return (int)(volume * Loudest / 100.0f + 0.5f);
        }

        /// <summary>A step of the fade: quieter in proportion to the time run, and stopped, with the timer ended, once FadeOut has run.</summary>
        private void Fade()
        {
            faded += FadeStep;
            if (playing == null || faded >= fadeOut - FadeStep * 0.001f)
            {
                Stop();
                return;
            }

            world!.SetLoopVolume(playing, (int)(fadeFrom * (1.0f - faded / fadeOut)));
        }

        /// <summary>Stops the sound and ends the fade's timer.</summary>
        private void Stop()
        {
            var timer = fader;
            fader = null;
            if (timer != null)
            {
                timer.End();
            }

            var stopped = playing;
            playing = null;
            if (stopped != null)
            {
                world!.StopLoop(stopped);
            }
        }
    }
}
