using System;

namespace Moonwell.Hosting
{
    /// <summary>The game as the input package sees it: the synced mouse events, and the machine-local reads and the changes for one player's screen. Types are the game's numbers.</summary>
    public interface IInputWorld
    {
        /// <summary>Starts delivering one player's mouse events: the three actions run when the mouse moves, a button goes down and a button comes up. Inside them, <c>EventPlayer</c>, <c>EventMouseX</c>, <c>EventMouseY</c> and <c>EventMouseButton</c> are the event's.</summary>
        void ListenMouse(player p, Action moved, Action pressed, Action released);

        /// <summary>The player of the mouse event being delivered.</summary>
        player EventPlayer();

        /// <summary>The x in the world the mouse event gave.</summary>
        float EventMouseX();

        /// <summary>The y in the world the mouse event gave.</summary>
        float EventMouseY();

        /// <summary>The button of the press or release event being delivered: 1 for left, 2 for middle, 3 for right, 0 for any other.</summary>
        int EventMouseButton();

        /// <summary>The top-right corner of the world's bounds.</summary>
        void WorldTopRight(out float maxX, out float maxY);

        /// <summary>The cursor's x on this machine's screen, in pixels.</summary>
        [MachineLocal]
        int MouseScreenX();

        /// <summary>The cursor's y on this machine's screen, in pixels.</summary>
        [MachineLocal]
        int MouseScreenY();

        /// <summary>The unit under the cursor on this machine now; null when none.</summary>
        [MachineLocal]
        unit? MouseFocusUnit();

        /// <summary>Whether a button (1 left, 2 middle, 3 right) is held on this machine now.</summary>
        [MachineLocal]
        bool IsMouseButtonPressed(int button);

        /// <summary>Puts a player's cursor at a screen position, on that player's machine only.</summary>
        void SetMouseScreen(player p, int x, int y);

        /// <summary>Shows or hides a player's cursor, on that player's machine only.</summary>
        void ShowCursor(player p, bool show);

        /// <summary>One field of this machine's camera, by Moonwell's code: 0 distance, 1 far Z, 2 angle of attack, 3 field of view, 4 roll, 5 rotation, 6 Z offset, 7 near Z, 8 local pitch, 9 local yaw, 10 local roll, 11 depth-of-field distance, 12 depth-of-field scale, 13 Z absolute. The game gives the angles in radians and the rest in game units.</summary>
        [MachineLocal]
        float CameraField(int field);

        /// <summary>The x this machine's camera looks at. After a move it reads the old place for the rest of the instant.</summary>
        [MachineLocal]
        float CameraTargetX();

        /// <summary>The y this machine's camera looks at.</summary>
        [MachineLocal]
        float CameraTargetY();

        /// <summary>The z this machine's camera looks at.</summary>
        [MachineLocal]
        float CameraTargetZ();

        /// <summary>The x of this machine's camera eye.</summary>
        [MachineLocal]
        float CameraEyeX();

        /// <summary>The y of this machine's camera eye.</summary>
        [MachineLocal]
        float CameraEyeY();

        /// <summary>The z of this machine's camera eye.</summary>
        [MachineLocal]
        float CameraEyeZ();

        /// <summary>Changes a field of a player's camera over some seconds, on that player's machine only. The code is as in CameraField; angles are in degrees.</summary>
        void SetCameraField(player p, int field, float value, float seconds);

        /// <summary>Pans a player's camera to a place over some seconds, on that player's machine only.</summary>
        void PanCamera(player p, float x, float y, float seconds);

        /// <summary>Locks a player's camera on a unit, on that player's machine only.</summary>
        void FollowUnit(player p, unit u);

        /// <summary>Ends a player's lock on a unit by setting the camera's position to its target, on that player's machine only.</summary>
        void StopFollowing(player p);

        /// <summary>Puts a player's camera back to the game camera over some seconds, on that player's machine only.</summary>
        void ResetCamera(player p, float seconds);

        /// <summary>Limits where a player's camera can go to a rectangle, on that player's machine only.</summary>
        void SetCameraBounds(player p, float minX, float minY, float maxX, float maxY);

        /// <summary>Makes a camera setup from one camera's numbers, starting at an offset: target x, target y, the count n, n pairs of field code and value, and the camera type (-1 for none), as the build writes them.</summary>
        camerasetup MakeSetup(float[] values, int offset);

        /// <summary>Applies a camera setup to a player's camera over some seconds, on that player's machine only.</summary>
        void ApplySetup(player p, camerasetup setup, float seconds);
    }
}
