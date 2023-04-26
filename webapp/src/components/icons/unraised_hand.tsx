// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {CSSProperties} from 'react';

type Props = {
    className?: string,
    fill?: string,
    style?: CSSProperties,
}

export default function UnraisedHandIcon(props: Props) {
    return (
        <svg
            {...props}
            viewBox='0.13 0.4 21 24'
            role='img'
        >
            <path d='M21.125 21.8688L1.39062 2.1344L0.125 3.40002L5.98438 9.30627V10.8063C5.64062 10.4625 5.23438 10.2438 4.76562 10.15L4.01562 9.91565C3.57812 9.79065 3.15625 9.80627 2.75 9.96252C2.375 10.1188 2.0625 10.3844 1.8125 10.7594C1.625 11.0406 1.51562 11.3531 1.48438 11.6969C1.48438 12.0406 1.54688 12.3688 1.67188 12.6813L4.25 19.15C4.875 20.7125 5.89062 21.9781 7.29688 22.9469C8.70312 23.9156 10.2656 24.4 11.9844 24.4C13.2344 24.4 14.3906 24.1344 15.4531 23.6031C16.5156 23.1031 17.4219 22.3844 18.1719 21.4469L19.8594 23.1344L21.125 21.8688ZM11.9844 22.3844C10.7031 22.3844 9.51562 22.025 8.42188 21.3063C7.35938 20.5875 6.57812 19.6188 6.07812 18.4L3.5 11.8375L4.01562 11.9781C4.51562 12.1031 4.84375 12.4156 5 12.9156L5.98438 15.4H8V11.275L16.7656 20.0406C16.2031 20.7594 15.5 21.3219 14.6562 21.7281C13.8438 22.1656 12.9531 22.3844 11.9844 22.3844ZM8 6.21252L6.07812 4.29065C6.23438 3.72815 6.53125 3.27502 6.96875 2.93127C7.40625 2.58752 7.92188 2.41565 8.51562 2.41565C8.76562 2.41565 8.9375 2.43127 9.03125 2.46252C9.15625 1.86877 9.4375 1.3844 9.875 1.0094C10.3438 0.603149 10.8906 0.400024 11.5156 0.400024C12.0156 0.400024 12.4688 0.556274 12.875 0.868774C13.3125 1.15002 13.625 1.52502 13.8125 1.99377C14.0312 1.93127 14.2656 1.90002 14.5156 1.90002C15.2031 1.90002 15.7812 2.15002 16.25 2.65002C16.75 3.11877 17 3.6969 17 4.3844V4.9469C17.0938 4.91565 17.2656 4.90002 17.5156 4.90002C18.2031 4.90002 18.7812 5.15002 19.25 5.65002C19.75 6.11877 20 6.6969 20 7.3844V16.3844C20 16.9469 19.9375 17.4938 19.8125 18.025L17.9844 16.1969V7.3844C17.9844 7.2594 17.9375 7.15002 17.8438 7.05627C17.75 6.96252 17.625 6.91565 17.4688 6.91565C17.3438 6.91565 17.2344 6.96252 17.1406 7.05627C17.0469 7.15002 17 7.2594 17 7.3844V12.4H14.9844V4.3844C14.9844 4.2594 14.9375 4.15002 14.8438 4.05627C14.75 3.96252 14.625 3.91565 14.4688 3.91565C14.3438 3.91565 14.2344 3.96252 14.1406 4.05627C14.0469 4.15002 14 4.2594 14 4.3844V12.2125L11.9844 10.1969V2.8844C11.9844 2.7594 11.9375 2.65002 11.8438 2.55627C11.75 2.46252 11.625 2.41565 11.4688 2.41565C11.3438 2.41565 11.2344 2.46252 11.1406 2.55627C11.0469 2.65002 11 2.7594 11 2.8844V9.21252L8.98438 7.1969V4.90002C8.98438 4.77502 8.9375 4.66565 8.84375 4.5719C8.75 4.4469 8.625 4.3844 8.46875 4.3844C8.34375 4.3844 8.23438 4.4469 8.14062 4.5719C8.04688 4.66565 8 4.77502 8 4.90002V6.21252Z'/>
        </svg>
    );
}
