package sdk_wrapper

import (
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vectorpb"
)

func FaceEnrollmentListAll() []*vectorpb.LoadedKnownFace {
	response, _ := Robot.Conn.RequestEnrolledNames(
		ctx,
		&vectorpb.RequestEnrolledNamesRequest{},
	)
	return response.GetFaces()
}

func FaceEnrollmentChangeName(faceId int32, oldName string, newName string) string {
	response, _ := Robot.Conn.UpdateEnrolledFaceByID(
		ctx,
		&vectorpb.UpdateEnrolledFaceByIDRequest{
			FaceId:  faceId,
			OldName: oldName,
			NewName: newName,
		},
	)
	return response.Status.String()
}

// Start face enrolling for person with the given name
// It doesn't seem to work, the face seems enrolled but not saved

func FaceEnrollmentStart(personName string) string {
	faces := FaceEnrollmentListAll()
	var maxId int32 = 0

	for i := 0; i < len(faces); i++ {
		if faces[i].FaceId > maxId {
			maxId = faces[i].FaceId
		}
	}
	maxId++

	response, _ := Robot.Conn.SetFaceToEnroll(
		ctx,
		&vectorpb.SetFaceToEnrollRequest{
			Name:        personName,
			ObservedId:  0,
			SaveId:      maxId,
			SaveToRobot: true,
			SayName:     true,
			UseMusic:    true,
		},
	)

	return response.Status.String()
}

// Cancels operation
func FaceEnrollmentCancel() string {
	response, _ := Robot.Conn.CancelFaceEnrollment(
		ctx,
		&vectorpb.CancelFaceEnrollmentRequest{},
	)
	return response.Status.String()
}

func FaceEnrollmentDeleteAll() string {
	response, _ := Robot.Conn.EraseAllEnrolledFaces(
		ctx,
		&vectorpb.EraseAllEnrolledFacesRequest{},
	)
	return response.Status.String()
}

func FaceEnrollmentDeleteById(id int32) string {
	response, _ := Robot.Conn.EraseEnrolledFaceByID(
		ctx,
		&vectorpb.EraseEnrolledFaceByIDRequest{
			FaceId: id,
		},
	)
	return response.Status.String()
}
