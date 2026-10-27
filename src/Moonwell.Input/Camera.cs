namespace Moonwell
{
    /// <summary>
    /// A player's camera. Get one with <c>player.Camera()</c>. Where a camera looks and how is that machine's alone, so the
    /// reads (the target, the eye, the distance and the angles) are read inside <c>Local.For</c>, and the game gives them a
    /// moment late: after a pan or a move, the target reads the old place until the next frame. A read is this machine's
    /// camera whichever player's camera it is read from: inside <c>Local.For(Player.One, ...)</c>,
    /// <c>Player.Two.Camera().Distance</c> reads Player One's. The changes (PanTo, the
    /// Set methods, Follow, StopFollowing, Apply, Reset, SetBounds) move only that player's camera and run only on that
    /// player's machine, so they are safe anywhere. Angles are in degrees. No event comes when a camera moves.
    /// </summary>
    public sealed class Camera
    {
        /// <summary>Makes the camera of a slot.</summary>
        internal Camera(int slot)
        {
            Slot = slot;
        }

        /// <summary>The slot of the camera's player.</summary>
        internal int Slot { get; }

        /// <summary>The player of this camera.</summary>
        [LocalSafe]
        public Player Player => Player.Get(Slot);

        /// <summary>The x the camera looks at, on this machine. After a pan or a move it reads the old place until the next frame. This machine's alone, so it is read inside Local.For.</summary>
        public float TargetX
        {
            [MachineLocal]
            get { return InputSystem.World.CameraTargetX(); }
        }

        /// <summary>The y the camera looks at, on this machine. After a pan or a move it reads the old place until the next frame. This machine's alone, so it is read inside Local.For.</summary>
        public float TargetY
        {
            [MachineLocal]
            get { return InputSystem.World.CameraTargetY(); }
        }

        /// <summary>The z the camera looks at, on this machine. After a pan or a move it reads the old place until the next frame. This machine's alone, so it is read inside Local.For.</summary>
        public float TargetZ
        {
            [MachineLocal]
            get { return InputSystem.World.CameraTargetZ(); }
        }

        /// <summary>The place the camera looks at, on this machine, as a new Vector3 each time (three native reads). After a pan or a move it reads the old place until the next frame. This machine's alone, so it is read inside Local.For.</summary>
        public Vector3 Target
        {
            [MachineLocal]
            get { return new Vector3(TargetX, TargetY, TargetZ); }
        }

        /// <summary>The x of the camera's eye, on this machine. This machine's alone, so it is read inside Local.For.</summary>
        public float EyeX
        {
            [MachineLocal]
            get { return InputSystem.World.CameraEyeX(); }
        }

        /// <summary>The y of the camera's eye, on this machine. This machine's alone, so it is read inside Local.For.</summary>
        public float EyeY
        {
            [MachineLocal]
            get { return InputSystem.World.CameraEyeY(); }
        }

        /// <summary>The z of the camera's eye, on this machine. This machine's alone, so it is read inside Local.For.</summary>
        public float EyeZ
        {
            [MachineLocal]
            get { return InputSystem.World.CameraEyeZ(); }
        }

        /// <summary>The place of the camera's eye, on this machine, as a new Vector3 each time (three native reads). This machine's alone, so it is read inside Local.For.</summary>
        public Vector3 Eye
        {
            [MachineLocal]
            get { return new Vector3(EyeX, EyeY, EyeZ); }
        }

        /// <summary>The distance from the camera to its target, on this machine. This machine's alone, so it is read inside Local.For.</summary>
        public float Distance
        {
            [MachineLocal]
            get { return InputSystem.World.CameraField(CameraFields.Distance); }
        }

        /// <summary>The camera's angle of attack in degrees, on this machine (the game gives radians; this converts). This machine's alone, so it is read inside Local.For.</summary>
        public float AngleOfAttack
        {
            [MachineLocal]
            get { return Angles.ToDegrees(InputSystem.World.CameraField(CameraFields.AngleOfAttack)); }
        }

        /// <summary>The camera's rotation in degrees, on this machine (the game gives radians; this converts). This machine's alone, so it is read inside Local.For.</summary>
        public float Rotation
        {
            [MachineLocal]
            get { return Angles.ToDegrees(InputSystem.World.CameraField(CameraFields.Rotation)); }
        }

        /// <summary>The camera's roll in degrees, on this machine (the game gives radians; this converts). This machine's alone, so it is read inside Local.For.</summary>
        public float Roll
        {
            [MachineLocal]
            get { return Angles.ToDegrees(InputSystem.World.CameraField(CameraFields.Roll)); }
        }

        /// <summary>The camera's field of view in degrees, on this machine (the game gives radians; this converts). This machine's alone, so it is read inside Local.For.</summary>
        public float FieldOfView
        {
            [MachineLocal]
            get { return Angles.ToDegrees(InputSystem.World.CameraField(CameraFields.FieldOfView)); }
        }

        /// <summary>The far clipping distance, on this machine. This machine's alone, so it is read inside Local.For.</summary>
        public float FarZ
        {
            [MachineLocal]
            get { return InputSystem.World.CameraField(CameraFields.FarZ); }
        }

        /// <summary>The camera's height offset above the ground, on this machine. This machine's alone, so it is read inside Local.For.</summary>
        public float ZOffset
        {
            [MachineLocal]
            get { return InputSystem.World.CameraField(CameraFields.ZOffset); }
        }

        /// <summary>Pans the player's camera to a place over some seconds (0 is at once). It moves only that player's camera, so it is safe anywhere. A read of the target right after still gives the old place.</summary>
        [LocalSafe]
        public void PanTo(Vector2 position, float seconds = 0f)
        {
            InputSystem.World.PanCamera(Player.Handle, position.X, position.Y, seconds);
        }

        /// <summary>Pans the player's camera to a place over some seconds (0 is at once). It moves only that player's camera, so it is safe anywhere. A read of the target right after still gives the old place.</summary>
        [LocalSafe]
        public void PanTo(float x, float y, float seconds)
        {
            InputSystem.World.PanCamera(Player.Handle, x, y, seconds);
        }

        /// <summary>Sets the distance to the target over some seconds. It changes only that player's camera, so it is safe anywhere.</summary>
        [LocalSafe]
        public void SetDistance(float distance, float seconds = 0f)
        {
            InputSystem.World.SetField(Player.Handle, CameraFields.Distance, distance, seconds);
        }

        /// <summary>Sets the angle of attack, in degrees, over some seconds. It changes only that player's camera, so it is safe anywhere.</summary>
        [LocalSafe]
        public void SetAngleOfAttack(float degrees, float seconds = 0f)
        {
            InputSystem.World.SetField(Player.Handle, CameraFields.AngleOfAttack, degrees, seconds);
        }

        /// <summary>Sets the rotation, in degrees, over some seconds. It changes only that player's camera, so it is safe anywhere.</summary>
        [LocalSafe]
        public void SetRotation(float degrees, float seconds = 0f)
        {
            InputSystem.World.SetField(Player.Handle, CameraFields.Rotation, degrees, seconds);
        }

        /// <summary>Sets the roll, in degrees, over some seconds. It changes only that player's camera, so it is safe anywhere.</summary>
        [LocalSafe]
        public void SetRoll(float degrees, float seconds = 0f)
        {
            InputSystem.World.SetField(Player.Handle, CameraFields.Roll, degrees, seconds);
        }

        /// <summary>Sets the field of view, in degrees, over some seconds. It changes only that player's camera, so it is safe anywhere.</summary>
        [LocalSafe]
        public void SetFieldOfView(float degrees, float seconds = 0f)
        {
            InputSystem.World.SetField(Player.Handle, CameraFields.FieldOfView, degrees, seconds);
        }

        /// <summary>Sets the far clipping distance over some seconds. It changes only that player's camera, so it is safe anywhere.</summary>
        [LocalSafe]
        public void SetFarZ(float distance, float seconds = 0f)
        {
            InputSystem.World.SetField(Player.Handle, CameraFields.FarZ, distance, seconds);
        }

        /// <summary>Sets the height offset over some seconds. It changes only that player's camera, so it is safe anywhere.</summary>
        [LocalSafe]
        public void SetZOffset(float offset, float seconds = 0f)
        {
            InputSystem.World.SetField(Player.Handle, CameraFields.ZOffset, offset, seconds);
        }

        /// <summary>Locks the player's camera on a unit, so it moves with it, until StopFollowing or a pan. It changes only that player's camera, so it is safe anywhere.</summary>
        [LocalSafe]
        public void Follow(Unit unit)
        {
            InputSystem.World.FollowUnit(Player.Handle, unit.Handle);
        }

        /// <summary>Ends a lock on a unit by setting the camera's position to where it looks now. It changes only that player's camera, so it is safe anywhere.</summary>
        [LocalSafe]
        public void StopFollowing()
        {
            InputSystem.World.StopFollowing(Player.Handle);
        }

        /// <summary>Applies a camera World Editor placed (see CameraSetup.Named) over some seconds. It changes only that player's camera, so it is safe anywhere.</summary>
        [LocalSafe]
        public void Apply(CameraSetup setup, float seconds = 0f)
        {
            InputSystem.World.ApplySetup(Player.Handle, setup.Handle, seconds);
        }

        /// <summary>Puts the player's camera back to the game camera over some seconds. It changes only that player's camera, so it is safe anywhere.</summary>
        [LocalSafe]
        public void Reset(float seconds = 0f)
        {
            InputSystem.World.ResetCamera(Player.Handle, seconds);
        }

        /// <summary>Limits where the player's camera can go to these bounds. It changes only that player's camera, so it is safe anywhere.</summary>
        [LocalSafe]
        public void SetBounds(Bounds bounds)
        {
            InputSystem.World.SetBounds(Player.Handle, bounds.Min.X, bounds.Min.Y, bounds.Max.X, bounds.Max.Y);
        }
    }
}
